// Package contact implements contact CRUD, scoped by the authenticated user.
package contact

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"referralautomation/internal/auth"
	"referralautomation/internal/database"
	"referralautomation/internal/db"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/httpx"
	"referralautomation/internal/platform/pagination"
	"referralautomation/internal/platform/validate"
)

// ---- model ----

type Contact struct {
	ID          uuid.UUID  `json:"id"`
	CompanyID   *uuid.UUID `json:"company_id"`
	CompanyName *string    `json:"company_name"`
	FullName    string     `json:"full_name"`
	LinkedInURL *string    `json:"linkedin_url"`
	Title       *string    `json:"title"`
	Email       *string    `json:"email"`
	Notes       *string    `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Input is the create/update request body (PUT replaces all fields).
type Input struct {
	CompanyID   *uuid.UUID `json:"company_id"`
	FullName    string     `json:"full_name"`
	LinkedInURL *string    `json:"linkedin_url"`
	Title       *string    `json:"title"`
	Email       *string    `json:"email"`
	Notes       *string    `json:"notes"`
}

type ListFilter struct {
	CompanyID *uuid.UUID
	Search    *string
}

// ---- repository ----

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID, in Input) (Contact, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Contact, error)
	List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Contact, int64, error)
	Update(ctx context.Context, userID, id uuid.UUID, in Input) (Contact, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type PgRepository struct{ q *db.Queries }

func NewPgRepository(q *db.Queries) *PgRepository { return &PgRepository{q: q} }

func build(id uuid.UUID, companyID *uuid.UUID, companyName *string, fullName string,
	linkedin, title, email, notes *string, created, updated time.Time) Contact {
	return Contact{ID: id, CompanyID: companyID, CompanyName: companyName, FullName: fullName,
		LinkedInURL: linkedin, Title: title, Email: email, Notes: notes, CreatedAt: created, UpdatedAt: updated}
}

func mapWriteErr(err error) error {
	switch {
	case database.IsUniqueViolation(err):
		return apperr.Conflict("a contact with this LinkedIn URL already exists")
	case database.IsForeignKeyViolation(err) && database.ConstraintName(err) == "contacts_company_fk":
		return apperr.Validation(map[string]string{"company_id": "company not found"})
	}
	return err
}

func (r *PgRepository) Create(ctx context.Context, userID uuid.UUID, in Input) (Contact, error) {
	id, err := r.q.CreateContact(ctx, db.CreateContactParams{
		UserID: userID, CompanyID: in.CompanyID, FullName: in.FullName, LinkedinURL: in.LinkedInURL,
		Title: in.Title, Email: in.Email, Notes: in.Notes,
	})
	if err != nil {
		return Contact{}, mapWriteErr(err)
	}
	return r.Get(ctx, userID, id)
}

func (r *PgRepository) Get(ctx context.Context, userID, id uuid.UUID) (Contact, error) {
	row, err := r.q.GetContact(ctx, db.GetContactParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Contact{}, apperr.NotFound("contact")
	}
	if err != nil {
		return Contact{}, err
	}
	return build(row.ID, row.CompanyID, row.CompanyName, row.FullName, row.LinkedinURL,
		row.Title, row.Email, row.Notes, row.CreatedAt, row.UpdatedAt), nil
}

func (r *PgRepository) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Contact, int64, error) {
	rows, err := r.q.ListContacts(ctx, db.ListContactsParams{
		UserID: userID, CompanyID: f.CompanyID, Search: f.Search,
		PageLimit: p.Limit(), PageOffset: p.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountContacts(ctx, db.CountContactsParams{
		UserID: userID, CompanyID: f.CompanyID, Search: f.Search,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Contact, 0, len(rows))
	for _, row := range rows {
		out = append(out, build(row.ID, row.CompanyID, row.CompanyName, row.FullName, row.LinkedinURL,
			row.Title, row.Email, row.Notes, row.CreatedAt, row.UpdatedAt))
	}
	return out, total, nil
}

func (r *PgRepository) Update(ctx context.Context, userID, id uuid.UUID, in Input) (Contact, error) {
	n, err := r.q.UpdateContact(ctx, db.UpdateContactParams{
		ID: id, UserID: userID, CompanyID: in.CompanyID, FullName: in.FullName, LinkedinURL: in.LinkedInURL,
		Title: in.Title, Email: in.Email, Notes: in.Notes,
	})
	if err != nil {
		return Contact{}, mapWriteErr(err)
	}
	if n == 0 {
		return Contact{}, apperr.NotFound("contact")
	}
	return r.Get(ctx, userID, id)
}

// Delete removes the contact and, via ON DELETE CASCADE, its job_contacts and their events.
func (r *PgRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	n, err := r.q.DeleteContact(ctx, db.DeleteContactParams{ID: id, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("contact")
	}
	return nil
}

// ---- service ----

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// normalizeLinkedIn canonicalizes a LinkedIn profile URL so duplicates are
// detected reliably: https, www.linkedin.com host, lower-case path, no query,
// fragment or trailing slash.
func normalizeLinkedIn(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host != "linkedin.com" && !strings.HasSuffix(host, ".linkedin.com") {
		return "", false
	}
	path := strings.TrimRight(strings.ToLower(u.Path), "/")
	if path == "" {
		return "", false
	}
	return "https://www.linkedin.com" + path, true
}

// NormalizeLinkedInURL is the exported canonicalizer (also used by Gmail matching).
func NormalizeLinkedInURL(raw string) (string, bool) { return normalizeLinkedIn(raw) }

func normalize(in Input) (Input, error) {
	errs := validate.Errors{}
	in.FullName = validate.Trim(in.FullName)
	validate.Required(errs, "full_name", in.FullName)
	validate.MaxLen(errs, "full_name", in.FullName, 200)

	in.LinkedInURL = validate.TrimPtr(in.LinkedInURL)
	if in.LinkedInURL != nil {
		if n, ok := normalizeLinkedIn(*in.LinkedInURL); ok {
			in.LinkedInURL = &n
		} else {
			errs.Add("linkedin_url", "must be a LinkedIn profile URL")
		}
	}
	in.Title = validate.TrimPtr(in.Title)
	validate.MaxLenPtr(errs, "title", in.Title, 200)
	in.Email = validate.TrimPtr(in.Email)
	validate.OptionalEmail(errs, "email", in.Email)
	in.Notes = validate.TrimPtr(in.Notes)
	validate.MaxLenPtr(errs, "notes", in.Notes, 5000)
	return in, errs.Err()
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, in Input) (Contact, error) {
	in, err := normalize(in)
	if err != nil {
		return Contact{}, err
	}
	return s.repo.Create(ctx, userID, in)
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Contact, error) {
	return s.repo.Get(ctx, userID, id)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Contact, int64, error) {
	if f.Search != nil {
		escaped := validate.EscapeLike(*f.Search)
		f.Search = &escaped
	}
	return s.repo.List(ctx, userID, f, p)
}

func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, in Input) (Contact, error) {
	in, err := normalize(in)
	if err != nil {
		return Contact{}, err
	}
	return s.repo.Update(ctx, userID, id, in)
}

func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}

// ---- handler ----

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r chi.Router) {
	r.Get("/", httpx.Wrap(h.list))
	r.Post("/", httpx.Wrap(h.create))
	r.Get("/{id}", httpx.Wrap(h.get))
	r.Put("/{id}", httpx.Wrap(h.update))
	r.Delete("/{id}", httpx.Wrap(h.delete))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	p, err := pagination.Parse(r)
	if err != nil {
		return err
	}
	companyID, err := httpx.QueryUUID(r, "company_id")
	if err != nil {
		return err
	}
	items, total, err := h.svc.List(r.Context(), userID,
		ListFilter{CompanyID: companyID, Search: httpx.QueryString(r, "q")}, p)
	if err != nil {
		return err
	}
	httpx.List(w, items, pagination.NewMeta(p, total))
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	var in Input
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	c, err := h.svc.Create(r.Context(), userID, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusCreated, c)
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	c, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, c)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in Input
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	c, err := h.svc.Update(r.Context(), userID, id, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, c)
	return nil
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), userID, id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}
