// Package config loads and validates configuration from the environment.
// A local .env file is read when present (development convenience only).
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env      string // development | production
	Port     string
	LogLevel string

	DatabaseURL          string // runtime connection (Supabase pooler is fine)
	MigrationDatabaseURL string // direct connection for migrations (optional)
	AutoMigrate          bool

	SupabaseURL string
	JWKSURL     string
	JWTIssuer   string
	JWTAudience string
	// JWTSecret enables legacy / local HS256 verification. Leave empty when the
	// Supabase project signs with asymmetric keys (the default today).
	JWTSecret string

	AllowedOrigins []string
	RequestTimeout time.Duration

	// Gmail integration (optional; all of the first four must be set to enable it).
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string // https://<api-host>/oauth/gmail/callback
	TokenEncryptionKey string // base64, 32 bytes (AES-256-GCM)
	FrontendURL        string // where the OAuth callback sends the browser afterwards
	GmailSyncMax       int    // max new messages processed per manual sync
	GmailLookbackDays  int    // window for the first sync

	RateLimitPerSecond float64
	RateLimitBurst     int
}

// GmailEnabled reports whether the Gmail integration is fully configured.
func (c Config) GmailEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" &&
		c.GoogleRedirectURL != "" && c.TokenEncryptionKey != ""
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// MigrationURL returns the URL used by migrations.
func (c Config) MigrationURL() string {
	if c.MigrationDatabaseURL != "" {
		return c.MigrationDatabaseURL
	}
	return c.DatabaseURL
}

func Load() (Config, error) {
	_ = godotenv.Load() // optional

	c := Config{
		Env:                  strings.ToLower(get("APP_ENV", "development")),
		Port:                 get("PORT", "8080"),
		LogLevel:             strings.ToLower(get("LOG_LEVEL", "info")),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		MigrationDatabaseURL: os.Getenv("MIGRATION_DATABASE_URL"),
		SupabaseURL:          strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		JWKSURL:              os.Getenv("SUPABASE_JWKS_URL"),
		JWTIssuer:            os.Getenv("SUPABASE_JWT_ISSUER"),
		JWTAudience:          get("SUPABASE_JWT_AUDIENCE", "authenticated"),
		JWTSecret:            os.Getenv("SUPABASE_JWT_SECRET"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		TokenEncryptionKey: os.Getenv("TOKEN_ENCRYPTION_KEY"),
		FrontendURL:        strings.TrimRight(get("FRONTEND_URL", "http://localhost:5173"), "/"),
	}

	var err error
	if c.AutoMigrate, err = getBool("AUTO_MIGRATE", false); err != nil {
		return c, err
	}
	secs, err := getInt("REQUEST_TIMEOUT_SECONDS", 30)
	if err != nil {
		return c, err
	}
	c.RequestTimeout = time.Duration(secs) * time.Second
	if c.GmailSyncMax, err = getInt("GMAIL_SYNC_MAX_MESSAGES", 50); err != nil {
		return c, err
	}
	if c.GmailLookbackDays, err = getInt("GMAIL_LOOKBACK_DAYS", 30); err != nil {
		return c, err
	}
	if c.RateLimitBurst, err = getInt("RATE_LIMIT_BURST", 60); err != nil {
		return c, err
	}
	rps, err := getInt("RATE_LIMIT_PER_SECOND", 20)
	if err != nil {
		return c, err
	}
	c.RateLimitPerSecond = float64(rps)

	defaultOrigins := ""
	if c.Env != "production" {
		defaultOrigins = "http://localhost:5173"
	}
	for _, o := range strings.Split(get("ALLOWED_ORIGINS", defaultOrigins), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, strings.TrimRight(o, "/"))
		}
	}

	if c.SupabaseURL != "" {
		if c.JWKSURL == "" {
			c.JWKSURL = c.SupabaseURL + "/auth/v1/.well-known/jwks.json"
		}
		if c.JWTIssuer == "" {
			c.JWTIssuer = c.SupabaseURL + "/auth/v1"
		}
	}

	return c, c.validate()
}

func (c Config) validate() error {
	var errs []error
	if c.Env != "development" && c.Env != "production" {
		errs = append(errs, errors.New("APP_ENV must be development or production"))
	}
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.JWTIssuer == "" {
		errs = append(errs, errors.New("SUPABASE_URL (or SUPABASE_JWT_ISSUER) is required"))
	}
	if c.JWKSURL == "" && c.JWTSecret == "" {
		errs = append(errs, errors.New("SUPABASE_URL (JWKS) or SUPABASE_JWT_SECRET is required to verify tokens"))
	}
	gmailParts := 0
	for _, v := range []string{c.GoogleClientID, c.GoogleClientSecret, c.GoogleRedirectURL, c.TokenEncryptionKey} {
		if v != "" {
			gmailParts++
		}
	}
	if gmailParts != 0 && gmailParts != 4 {
		errs = append(errs, errors.New("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URL and TOKEN_ENCRYPTION_KEY must be set together"))
	}
	if c.TokenEncryptionKey != "" {
		if key, err := base64.StdEncoding.DecodeString(c.TokenEncryptionKey); err != nil || len(key) != 32 {
			errs = append(errs, errors.New("TOKEN_ENCRYPTION_KEY must be base64 of exactly 32 bytes (openssl rand -base64 32)"))
		}
	}
	if c.IsProduction() {
		if c.GmailEnabled() && !strings.HasPrefix(c.GoogleRedirectURL, "https://") {
			errs = append(errs, errors.New("GOOGLE_REDIRECT_URL must be https in production"))
		}
		if len(c.AllowedOrigins) == 0 {
			errs = append(errs, errors.New("ALLOWED_ORIGINS is required in production"))
		}
		for _, o := range c.AllowedOrigins {
			if o == "*" {
				errs = append(errs, errors.New("ALLOWED_ORIGINS must not contain * in production"))
			}
		}
	}
	return errors.Join(errs...)
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getBool(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return b, nil
}

func getInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}
