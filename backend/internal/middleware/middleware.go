// Package middleware contains HTTP middleware: authentication, user
// provisioning, request logging, panic recovery and security headers.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"referralautomation/internal/auth"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/httpx"
)

// Authenticate requires a valid Supabase bearer token and stores the
// Principal in the request context. Tokens are never logged.
func Authenticate(v *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			token = strings.TrimSpace(token)
			if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
				unauthorized(w, r, "missing or malformed Authorization header")
				return
			}
			claims, err := v.Verify(r.Context(), token)
			if err != nil {
				slog.DebugContext(r.Context(), "token rejected", "reason", err.Error())
				unauthorized(w, r, "invalid or expired token")
				return
			}
			ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: claims.UserID, Email: claims.Email})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter, r *http.Request, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	httpx.WriteError(w, r, apperr.Unauthorized(msg))
}

// Provisioner makes sure a users row exists for an authenticated identity.
type Provisioner interface {
	Ensure(ctx context.Context, id uuid.UUID, email string) error
}

// ProvisionUser must run after Authenticate.
func ProvisionUser(p Provisioner) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				unauthorized(w, r, "authentication required")
				return
			}
			if err := p.Ensure(r.Context(), principal.UserID, principal.Email); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLogger emits one structured log line per request. It logs the path
// only (no query string, headers or bodies) so tokens and message contents
// can never leak into logs.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := chimw.GetReqID(r.Context())
		if reqID != "" {
			w.Header().Set("X-Request-ID", reqID)
		}
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		slog.LogAttrs(r.Context(), level, "http request",
			slog.String("request_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int("bytes", ww.BytesWritten()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}

// Recoverer turns panics into a JSON 500 and logs the stack.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.ErrorContext(r.Context(), "panic recovered",
					"panic", rec,
					"request_id", chimw.GetReqID(r.Context()),
					"stack", string(debug.Stack()),
				)
				httpx.WriteError(w, r, apperr.New(http.StatusInternalServerError, "internal_error", "internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sets conservative defaults for an API-only service.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
