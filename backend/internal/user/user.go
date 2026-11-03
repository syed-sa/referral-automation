// Package user provisions application users from Supabase identities and
// serves GET /me.
package user

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"referralautomation/internal/auth"
	"referralautomation/internal/db"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/httpx"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName *string   `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Repository struct{ q *db.Queries }

func NewRepository(q *db.Queries) *Repository { return &Repository{q: q} }

func (r *Repository) Upsert(ctx context.Context, id uuid.UUID, email string) error {
	return r.q.UpsertUser(ctx, db.UpsertUserParams{ID: id, Email: email})
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.NotFound("user")
	}
	if err != nil {
		return User{}, err
	}
	return User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

// Service provisions users lazily on first authenticated request. A small
// in-process cache avoids an upsert on every request; it is only an
// optimization (the upsert is idempotent).
type Service struct {
	repo *Repository

	mu    sync.RWMutex
	known map[uuid.UUID]string // user id -> last synced email
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo, known: map[uuid.UUID]string{}}
}

func (s *Service) Ensure(ctx context.Context, id uuid.UUID, email string) error {
	s.mu.RLock()
	cached, ok := s.known[id]
	s.mu.RUnlock()
	if ok && (email == "" || cached == email) {
		return nil
	}
	if err := s.repo.Upsert(ctx, id, email); err != nil {
		return err
	}
	s.mu.Lock()
	if len(s.known) > 10_000 {
		s.known = map[uuid.UUID]string{}
	}
	s.known[id] = email
	s.mu.Unlock()
	return nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.Get(ctx, id)
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r chi.Router) {
	r.Get("/me", httpx.Wrap(h.me))
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	u, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, u)
	return nil
}
