// Package job implements job CRUD, scoped by the authenticated user.
package job

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

const (
	StatusActive = "ACTIVE"
	StatusClosed = "CLOSED"
)

// ---- model ----

type Job struct {
	ID          uuid.UUID `json:"id"`
	CompanyID   uuid.UUID `json:"company_id"`
	CompanyName string    `json:"company_name"`
	Title       string    `json:"title"`
	JobURL      *string   `json:"job_url"`
	Location    *string   `json:"location"`
	Status      string    `json:"status"`
	Notes       *string   `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Input is the create/update request body (PUT replaces all fields).
type Input struct {
	CompanyID uuid.UUID `json:"company_id"`
	Title     string    `json:"title"`
	JobURL    *string   `json:"job_url"`
	Location  *string   `json:"location"`
	Status    string    `json:"status"`
	Notes     *string   `json:"notes"`
}

type ListFilter struct {
	CompanyID *uuid.UUID
	Status    *string
	Search    *string
}

// ---- repository ----

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID, in Input) (Job, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Job, error)
	List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Job, int64, error)
	Update(ctx context.Context, userID, id uuid.UUID, in Input) (Job, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type PgRepository struct{ q *db.Queries }

func NewPgRepository(q *db.Queries) *PgRepository { return &PgRepository{q: q} }

func build(id, companyID uuid.UUID, companyName, title string, jobURL, location *string,
	status string, notes *string, created, updated time.Time) Job {
	return Job{ID: id, CompanyID: companyID, CompanyName: companyName, Title: title, JobURL: jobURL,
		Location: location, Status: status, Notes: notes, CreatedAt: created, UpdatedAt: updated}
}

// mapWriteErr turns the composite-FK failure into a validation error: the
// referenced company either does not exist or belongs to another user.
func mapWriteErr(err error) error {
	if database.IsForeignKeyViolation(err) && database.ConstraintName(err) == "jobs_company_fk" {
		return apperr.Validation(map[string]string{"company_id": "company not found"})
	}
	return err
}

func (r *PgRepository) Create(ctx context.Context, userID uuid.UUID, in Input) (Job, error) {
	id, err := r.q.CreateJob(ctx, db.CreateJobParams{
		UserID: userID, CompanyID: in.CompanyID, Title: in.Title, JobURL: in.JobURL,
		Location: in.Location, Status: in.Status, Notes: in.Notes,
	})
	if err != nil {
		return Job{}, mapWriteErr(err)
	}
	return r.Get(ctx, userID, id)
}

func (r *PgRepository) Get(ctx context.Context, userID, id uuid.UUID) (Job, error) {
	row, err := r.q.GetJob(ctx, db.GetJobParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, apperr.NotFound("job")
	}
	if err != nil {
		return Job{}, err
	}
	return build(row.ID, row.CompanyID, row.CompanyName, row.Title, row.JobURL, row.Location,
		row.Status, row.Notes, row.CreatedAt, row.UpdatedAt), nil
}

func (r *PgRepository) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Job, int64, error) {
	rows, err := r.q.ListJobs(ctx, db.ListJobsParams{
		UserID: userID, CompanyID: f.CompanyID, Status: f.Status, Search: f.Search,
		PageLimit: p.Limit(), PageOffset: p.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountJobs(ctx, db.CountJobsParams{
		UserID: userID, CompanyID: f.CompanyID, Status: f.Status, Search: f.Search,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Job, 0, len(rows))
	for _, row := range rows {
		out = append(out, build(row.ID, row.CompanyID, row.CompanyName, row.Title, row.JobURL, row.Location,
			row.Status, row.Notes, row.CreatedAt, row.UpdatedAt))
	}
	return out, total, nil
}

func (r *PgRepository) Update(ctx context.Context, userID, id uuid.UUID, in Input) (Job, error) {
	n, err := r.q.UpdateJob(ctx, db.UpdateJobParams{
		ID: id, UserID: userID, CompanyID: in.CompanyID, Title: in.Title, JobURL: in.JobURL,
		Location: in.Location, Status: in.Status, Notes: in.Notes,
	})
	if err != nil {
		return Job{}, mapWriteErr(err)
	}
	if n == 0 {
		return Job{}, apperr.NotFound("job")
	}
	return r.Get(ctx, userID, id)
}

// Delete removes the job and, via ON DELETE CASCADE, its job_contacts and their events.
func (r *PgRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	n, err := r.q.DeleteJob(ctx, db.DeleteJobParams{ID: id, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("job")
	}
	return nil
}

// ---- service ----

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func normalize(in Input) (Input, error) {
	errs := validate.Errors{}
	if in.CompanyID == uuid.Nil {
		errs.Add("company_id", "is required")
	}
	in.Title = validate.Trim(in.Title)
	validate.Required(errs, "title", in.Title)
	validate.MaxLen(errs, "title", in.Title, 200)
	in.JobURL = validate.TrimPtr(in.JobURL)
	validate.OptionalURL(errs, "job_url", in.JobURL)
	in.Location = validate.TrimPtr(in.Location)
	validate.MaxLenPtr(errs, "location", in.Location, 200)
	in.Notes = validate.TrimPtr(in.Notes)
	validate.MaxLenPtr(errs, "notes", in.Notes, 5000)
	if in.Status == "" {
		in.Status = StatusActive
	}
	validate.OneOf(errs, "status", in.Status, StatusActive, StatusClosed)
	return in, errs.Err()
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, in Input) (Job, error) {
	in, err := normalize(in)
	if err != nil {
		return Job{}, err
	}
	return s.repo.Create(ctx, userID, in)
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Job, error) {
	return s.repo.Get(ctx, userID, id)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]Job, int64, error) {
	var errs = validate.Errors{}
	if f.Status != nil {
		validate.OneOf(errs, "status", *f.Status, StatusActive, StatusClosed)
	}
	if err := errs.Err(); err != nil {
		return nil, 0, err
	}
	if f.Search != nil {
		escaped := validate.EscapeLike(*f.Search)
		f.Search = &escaped
	}
	return s.repo.List(ctx, userID, f, p)
}

func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, in Input) (Job, error) {
	in, err := normalize(in)
	if err != nil {
		return Job{}, err
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
	f := ListFilter{CompanyID: companyID, Status: httpx.QueryString(r, "status"), Search: httpx.QueryString(r, "q")}
	items, total, err := h.svc.List(r.Context(), userID, f, p)
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
	j, err := h.svc.Create(r.Context(), userID, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusCreated, j)
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
	j, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, j)
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
	j, err := h.svc.Update(r.Context(), userID, id, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, j)
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
