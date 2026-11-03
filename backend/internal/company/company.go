// Package company implements company CRUD. Every operation is scoped by the
// authenticated user's id, which is passed explicitly down to the SQL layer.
package company

import (
	"context"
	"errors"
	"net/http"
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

type Company struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Website      *string   `json:"website"`
	Industry     *string   `json:"industry"`
	Location     *string   `json:"location"`
	Notes        *string   `json:"notes"`
	JobCount     int64     `json:"job_count"`
	ContactCount int64     `json:"contact_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Input is the create/update request body (PUT replaces all fields).
type Input struct {
	Name     string  `json:"name"`
	Website  *string `json:"website"`
	Industry *string `json:"industry"`
	Location *string `json:"location"`
	Notes    *string `json:"notes"`
}

type ListFilter struct{ Search *string }

// ---- repository ----

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID, in Input) (Company, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Company, error)
	List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Company, int64, error)
	Update(ctx context.Context, userID, id uuid.UUID, in Input) (Company, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type PgRepository struct{ q *db.Queries }

func NewPgRepository(q *db.Queries) *PgRepository { return &PgRepository{q: q} }

func build(id uuid.UUID, name string, website, industry, location, notes *string,
	jobs, contacts int64, created, updated time.Time) Company {
	return Company{ID: id, Name: name, Website: website, Industry: industry, Location: location,
		Notes: notes, JobCount: jobs, ContactCount: contacts, CreatedAt: created, UpdatedAt: updated}
}

func mapWriteErr(err error) error {
	if database.IsUniqueViolation(err) {
		return apperr.Conflict("a company with this name already exists")
	}
	return err
}

func (r *PgRepository) Create(ctx context.Context, userID uuid.UUID, in Input) (Company, error) {
	id, err := r.q.CreateCompany(ctx, db.CreateCompanyParams{
		UserID: userID, Name: in.Name, Website: in.Website, Industry: in.Industry,
		Location: in.Location, Notes: in.Notes,
	})
	if err != nil {
		return Company{}, mapWriteErr(err)
	}
	return r.Get(ctx, userID, id)
}

func (r *PgRepository) Get(ctx context.Context, userID, id uuid.UUID) (Company, error) {
	row, err := r.q.GetCompany(ctx, db.GetCompanyParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Company{}, apperr.NotFound("company")
	}
	if err != nil {
		return Company{}, err
	}
	return build(row.ID, row.Name, row.Website, row.Industry, row.Location, row.Notes,
		row.JobCount, row.ContactCount, row.CreatedAt, row.UpdatedAt), nil
}

func (r *PgRepository) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Company, int64, error) {
	rows, err := r.q.ListCompanies(ctx, db.ListCompaniesParams{
		UserID: userID, Search: f.Search, PageLimit: p.Limit(), PageOffset: p.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountCompanies(ctx, db.CountCompaniesParams{UserID: userID, Search: f.Search})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Company, 0, len(rows))
	for _, row := range rows {
		out = append(out, build(row.ID, row.Name, row.Website, row.Industry, row.Location, row.Notes,
			row.JobCount, row.ContactCount, row.CreatedAt, row.UpdatedAt))
	}
	return out, total, nil
}

func (r *PgRepository) Update(ctx context.Context, userID, id uuid.UUID, in Input) (Company, error) {
	n, err := r.q.UpdateCompany(ctx, db.UpdateCompanyParams{
		ID: id, UserID: userID, Name: in.Name, Website: in.Website, Industry: in.Industry,
		Location: in.Location, Notes: in.Notes,
	})
	if err != nil {
		return Company{}, mapWriteErr(err)
	}
	if n == 0 {
		return Company{}, apperr.NotFound("company")
	}
	return r.Get(ctx, userID, id)
}

func (r *PgRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	n, err := r.q.DeleteCompany(ctx, db.DeleteCompanyParams{ID: id, UserID: userID})
	if database.IsForeignKeyViolation(err) {
		return apperr.Conflict("company still has jobs or contacts; remove or reassign them first")
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("company")
	}
	return nil
}

// ---- service ----

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func normalize(in Input) (Input, error) {
	errs := validate.Errors{}
	in.Name = validate.Trim(in.Name)
	validate.Required(errs, "name", in.Name)
	validate.MaxLen(errs, "name", in.Name, 200)
	in.Website = validate.TrimPtr(in.Website)
	validate.OptionalURL(errs, "website", in.Website)
	in.Industry = validate.TrimPtr(in.Industry)
	validate.MaxLenPtr(errs, "industry", in.Industry, 100)
	in.Location = validate.TrimPtr(in.Location)
	validate.MaxLenPtr(errs, "location", in.Location, 200)
	in.Notes = validate.TrimPtr(in.Notes)
	validate.MaxLenPtr(errs, "notes", in.Notes, 5000)
	return in, errs.Err()
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, in Input) (Company, error) {
	in, err := normalize(in)
	if err != nil {
		return Company{}, err
	}
	return s.repo.Create(ctx, userID, in)
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Company, error) {
	return s.repo.Get(ctx, userID, id)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Company, int64, error) {
	if f.Search != nil {
		escaped := validate.EscapeLike(*f.Search)
		f.Search = &escaped
	}
	return s.repo.List(ctx, userID, f, p)
}

func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, in Input) (Company, error) {
	in, err := normalize(in)
	if err != nil {
		return Company{}, err
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
	items, total, err := h.svc.List(r.Context(), userID, ListFilter{Search: httpx.QueryString(r, "q")}, p)
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
