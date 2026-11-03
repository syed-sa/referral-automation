package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid token")

const (
	keyCacheTTL        = 10 * time.Minute
	minRefreshInterval = 30 * time.Second
	jwksFetchTimeout   = 10 * time.Second
)

type VerifierConfig struct {
	JWKSURL  string
	Issuer   string
	Audience string
	// HMACSecret enables HS256 verification (legacy Supabase projects / local dev).
	// When empty, HS* tokens are rejected outright.
	HMACSecret string
}

// Claims are the validated claims the application cares about.
type Claims struct {
	UserID uuid.UUID
	Email  string
}

type supabaseClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Verifier validates Supabase access tokens. The signing algorithm is NOT
// assumed: asymmetric keys (ES256/RS256/EdDSA...) are looked up in the JWKS by
// `kid`, and HS256 is accepted only when a secret is configured. The key
// handed to the JWT library is always derived from the token's declared
// algorithm family, so an attacker cannot make an RSA public key act as an
// HMAC secret (algorithm confusion).
type Verifier struct {
	cfg    VerifierConfig
	parser *jwt.Parser
	client *http.Client

	refreshMu   sync.Mutex // serializes JWKS fetches
	mu          sync.RWMutex
	keys        map[string]crypto.PublicKey
	fetchedAt   time.Time
	lastAttempt time.Time
}

func NewVerifier(cfg VerifierConfig) *Verifier {
	methods := []string{"ES256", "ES384", "ES512", "RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "EdDSA"}
	if cfg.HMACSecret != "" {
		methods = append(methods, "HS256")
	}
	return &Verifier{
		cfg: cfg,
		parser: jwt.NewParser(
			jwt.WithValidMethods(methods),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(10*time.Second),
		),
		client: &http.Client{Timeout: jwksFetchTimeout},
		keys:   map[string]crypto.PublicKey{},
	}
}

// Verify validates signature, issuer, audience and expiry and returns the claims.
// The raw token is never logged or included in returned errors.
func (v *Verifier) Verify(ctx context.Context, raw string) (Claims, error) {
	var sc supabaseClaims
	_, err := v.parser.ParseWithClaims(raw, &sc, func(t *jwt.Token) (any, error) {
		return v.keyFor(ctx, t)
	})
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if sc.Role != "authenticated" {
		return Claims{}, fmt.Errorf("%w: unexpected role", ErrInvalidToken)
	}
	id, err := uuid.Parse(sc.Subject)
	if err != nil || id == uuid.Nil {
		return Claims{}, fmt.Errorf("%w: invalid subject", ErrInvalidToken)
	}
	return Claims{UserID: id, Email: sc.Email}, nil
}

func (v *Verifier) keyFor(ctx context.Context, t *jwt.Token) (any, error) {
	switch t.Method.Alg() {
	case "HS256":
		if v.cfg.HMACSecret == "" {
			return nil, errors.New("HMAC tokens are not accepted")
		}
		return []byte(v.cfg.HMACSecret), nil
	default:
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token has no kid header")
		}
		return v.publicKey(ctx, kid)
	}
}

func (v *Verifier) publicKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	if v.cfg.JWKSURL == "" {
		return nil, errors.New("JWKS is not configured")
	}

	k, ok, fresh := v.lookup(kid)
	if ok && fresh {
		return k, nil
	}
	// Unknown kid or stale cache: refresh (rate limited), then look again.
	if err := v.refresh(ctx); err != nil && ok {
		return k, nil // serve the stale key if the refresh failed
	} else if err != nil {
		return nil, err
	}
	if k, ok, _ = v.lookup(kid); ok {
		return k, nil
	}
	return nil, errors.New("unknown signing key")
}

func (v *Verifier) lookup(kid string) (key crypto.PublicKey, ok, fresh bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keys[kid]
	return key, ok, time.Since(v.fetchedAt) < keyCacheTTL
}

func (v *Verifier) refresh(ctx context.Context) error {
	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()

	v.mu.RLock()
	last := v.lastAttempt
	v.mu.RUnlock()
	if !last.IsZero() && time.Since(last) < minRefreshInterval {
		return nil
	}
	v.mu.Lock()
	v.lastAttempt = time.Now()
	v.mu.Unlock()

	// Detach from the request's cancellation so one aborted request does not
	// poison the shared cache refresh.
	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jwksFetchTimeout)
	defer cancel()
	keys, err := v.fetchJWKS(fetchCtx)
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *Verifier) fetchJWKS(ctx context.Context) (map[string]crypto.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch JWKS: status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}
	keys := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kid == "" || k.Use == "enc" {
			continue
		}
		pub, err := k.publicKey()
		if err != nil {
			continue // skip keys we cannot use
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, errors.New("JWKS contains no usable keys")
	}
	return keys, nil
}

func (k jwk) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "EC":
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		x, err := b64(k.X)
		if err != nil {
			return nil, err
		}
		y, err := b64(k.Y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
	case "RSA":
		n, err := b64(k.N)
		if err != nil {
			return nil, err
		}
		e, err := b64(k.E)
		if err != nil {
			return nil, err
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, fmt.Errorf("unsupported OKP curve %q", k.Crv)
		}
		x, err := b64(k.X)
		if err != nil {
			return nil, err
		}
		if len(x) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 key length")
		}
		return ed25519.PublicKey(x), nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

func b64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
