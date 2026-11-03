// Package server wires dependencies and builds the HTTP router.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"referralautomation/internal/auth"
	"referralautomation/internal/company"
	"referralautomation/internal/contact"
	"referralautomation/internal/dashboard"
	"referralautomation/internal/db"
	"referralautomation/internal/followup"
	"referralautomation/internal/job"
	"referralautomation/internal/jobcontact"
	"referralautomation/internal/mailsync"
	"referralautomation/internal/middleware"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/config"
	"referralautomation/internal/platform/httpx"
	"referralautomation/internal/secretbox"
	"referralautomation/internal/user"
)

type Deps struct {
	Config   config.Config
	Pool     *pgxpool.Pool
	Verifier *auth.Verifier
}

func New(d Deps) (http.Handler, error) {
	q := db.New(d.Pool)

	userSvc := user.NewService(user.NewRepository(q))
	userH := user.NewHandler(userSvc)
	companyH := company.NewHandler(company.NewService(company.NewPgRepository(q)))
	jobH := job.NewHandler(job.NewService(job.NewPgRepository(q)))
	contactH := contact.NewHandler(contact.NewService(contact.NewPgRepository(q)))
	jobContactH := jobcontact.NewHandler(jobcontact.NewService(jobcontact.NewPgRepository(d.Pool, q)))
	dashboardH := dashboard.NewHandler(dashboard.NewService(q))
	followupH := followup.NewHandler(followup.NewService(d.Pool, q))

	// Gmail is optional: without Google credentials its endpoints answer 503.
	var gmailSvc *mailsync.Service
	if d.Config.GmailEnabled() {
		box, err := secretbox.New(d.Config.TokenEncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("token encryption key: %w", err)
		}
		gmailSvc = mailsync.NewService(d.Pool, q, mailsync.Config{
			ClientID:     d.Config.GoogleClientID,
			ClientSecret: d.Config.GoogleClientSecret,
			RedirectURL:  d.Config.GoogleRedirectURL,
			FrontendURL:  d.Config.FrontendURL,
			SyncMax:      d.Config.GmailSyncMax,
			LookbackDays: d.Config.GmailLookbackDays,
		}, box)
	}
	gmailH := mailsync.NewHandler(gmailSvc)

	limiter := middleware.NewRateLimiter(d.Config.RateLimitPerSecond, d.Config.RateLimitBurst)
	gmailLimiter := middleware.NewRateLimiter(0.2, 5) // sync/connect are expensive

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.Config.AllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           600,
	}))
	r.Use(chimw.Timeout(d.Config.RequestTimeout))

	r.NotFound(httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return apperr.NotFound("route")
	}))
	r.MethodNotAllowed(httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return apperr.New(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}))

	// Unauthenticated health endpoints (no data access beyond a DB ping).
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Pool.Ping(ctx); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	// Public OAuth redirect target: identity comes from the single-use state, not a JWT.
	gmailH.RegisterCallback(r)

	// Everything below requires a valid Supabase JWT. The user id used by every
	// handler is taken from the validated token, never from the request.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Authenticate(d.Verifier))
		r.Use(middleware.ProvisionUser(userSvc))
		r.Use(limiter.PerUser)

		userH.Register(r)
		dashboardH.Register(r)
		followupH.RegisterRoot(r)
		r.Route("/gmail", func(r chi.Router) {
			r.Use(gmailLimiter.PerUser)
			gmailH.Register(r)
		})
		r.Route("/companies", companyH.Register)
		r.Route("/jobs", func(r chi.Router) {
			jobH.Register(r)
			jobContactH.RegisterJobScoped(r)
		})
		r.Route("/contacts", contactH.Register)
		r.Route("/job-contacts", func(r chi.Router) {
			jobContactH.Register(r)
			followupH.RegisterJobContactScoped(r)
		})
	})

	return r, nil
}
