// Package jobcontact manages referral attempts: a contact linked to a job.
// Each job_contact carries the workflow status (see internal/workflow).
package jobcontact

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"referralautomation/internal/auth"
	"referralautomation/internal/database"
	"referralautomation/internal/db"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/httpx"
	"referralautomation/internal/platform/pagination"
	"referralautomation/internal/platform/validate"
	"referralautomation/internal/workflow"
)

// ---- model ----

type JobContact struct {
	ID                 uuid.UUID  `json:"id"`
	JobID              uuid.UUID  `json:"job_id"`
	JobTitle           string     `json:"job_title"`
	CompanyID          uuid.UUID  `json:"company_id"`
	CompanyName        string     `json:"company_name"`
	ContactID          uuid.UUID  `json:"contact_id"`
	ContactName        string     `json:"contact_name"`
	ContactLinkedInURL *string    `json:"contact_linkedin_url"`
	Status             string     `json:"status"`
	InvitationSentAt   *time.Time `json:"invitation_sent_at"`
	AcceptedAt         *time.Time `json:"accepted_at"`
	MessageSentAt      *time.Time `json:"message_sent_at"`
	FollowUpDueAt      *time.Time `json:"followup_due_at"`
	ReferralReceivedAt *time.Time `json:"referral_received_at"`
	Notes              *string    `json:"notes"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	AllowedNext        []string   `json:"allowed_next_statuses"`
}

// TransitionInput is the body of POST /job-contacts/{id}/transitions.
type TransitionInput struct {
	ToStatus   string     `json:"to_status"`
	OccurredAt *time.Time `json:"occurred_at"` // optional: when it really happened
	Note       *string    `json:"note"`
}

// UpdateInput is the body of PATCH /job-contacts/{id}.
type UpdateInput struct {
	Notes *string `json:"notes"`
}

type Event struct {
	ID         uuid.UUID       `json:"id"`
	EventType  string          `json:"event_type"`
	FromStatus *string         `json:"from_status"`
	ToStatus   *string         `json:"to_status"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

// AddInput is the body of POST /jobs/{id}/contacts. The job id comes from the
// path; the owner always comes from the JWT.
type AddInput struct {
	ContactID uuid.UUID `json:"contact_id"`
	Notes     *string   `json:"notes"`
}

type ListFilter struct {
	JobID     *uuid.UUID
	ContactID *uuid.UUID
	Status    *string
}

// ---- repository ----

type Repository interface {
	Create(ctx context.Context, userID, jobID uuid.UUID, in AddInput) (JobContact, error)
	Get(ctx context.Context, userID, id uuid.UUID) (JobContact, error)
	List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]JobContact, int64, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
	Events(ctx context.Context, userID, id uuid.UUID) ([]Event, error)
	Transition(ctx context.Context, userID, id uuid.UUID, to workflow.Status, opts TransitionOptions) (JobContact, error)
	UpdateNotes(ctx context.Context, userID, id uuid.UUID, notes *string) (JobContact, error)
}

type PgRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewPgRepository(pool *pgxpool.Pool, q *db.Queries) *PgRepository {
	return &PgRepository{pool: pool, q: q}
}

// Create inserts the job_contact and its first workflow event atomically.
func (r *PgRepository) Create(ctx context.Context, userID, jobID uuid.UUID, in AddInput) (JobContact, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return JobContact{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	qtx := r.q.WithTx(tx)
	id, err := qtx.CreateJobContact(ctx, db.CreateJobContactParams{
		UserID: userID, JobID: jobID, ContactID: in.ContactID, Notes: in.Notes,
	})
	if err != nil {
		return JobContact{}, mapCreateErr(err)
	}
	to := string(workflow.StatusAdded)
	if err := qtx.CreateWorkflowEvent(ctx, db.CreateWorkflowEventParams{
		UserID:       userID,
		JobContactID: id,
		EventType:    string(workflow.EventJobContactCreated),
		ToStatus:     &to,
		Metadata:     []byte("{}"),
	}); err != nil {
		return JobContact{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return JobContact{}, err
	}
	return r.Get(ctx, userID, id)
}

// mapCreateErr distinguishes which composite FK failed. Both mean "not yours or
// does not exist": a missing job is a 404 (it is the URL resource), a missing
// contact is a 422 (it is a body field).
func mapCreateErr(err error) error {
	switch {
	case database.IsUniqueViolation(err):
		return apperr.Conflict("this contact is already linked to this job")
	case database.IsForeignKeyViolation(err):
		switch database.ConstraintName(err) {
		case "job_contacts_job_fk":
			return apperr.NotFound("job")
		case "job_contacts_contact_fk":
			return apperr.Validation(map[string]string{"contact_id": "contact not found"})
		}
	}
	return err
}

func build(id, jobID uuid.UUID, jobTitle string, companyID uuid.UUID, companyName string,
	contactID uuid.UUID, contactName string, contactLinkedIn *string, status string,
	invitationSent, accepted, messageSent, followUpDue, referralReceived *time.Time,
	notes *string, created, updated time.Time) JobContact {
	next := workflow.AllowedNext(workflow.Status(status))
	allowed := make([]string, 0, len(next))
	for _, n := range next {
		allowed = append(allowed, string(n))
	}
	return JobContact{
		AllowedNext: allowed,
		ID:          id, JobID: jobID, JobTitle: jobTitle, CompanyID: companyID, CompanyName: companyName,
		ContactID: contactID, ContactName: contactName, ContactLinkedInURL: contactLinkedIn,
		Status: status, InvitationSentAt: invitationSent, AcceptedAt: accepted,
		MessageSentAt: messageSent, FollowUpDueAt: followUpDue, ReferralReceivedAt: referralReceived,
		Notes: notes, CreatedAt: created, UpdatedAt: updated,
	}
}

func (r *PgRepository) Get(ctx context.Context, userID, id uuid.UUID) (JobContact, error) {
	row, err := r.q.GetJobContact(ctx, db.GetJobContactParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return JobContact{}, apperr.NotFound("job contact")
	}
	if err != nil {
		return JobContact{}, err
	}
	return build(row.ID, row.JobID, row.JobTitle, row.CompanyID, row.CompanyName,
		row.ContactID, row.ContactName, row.ContactLinkedinURL, row.Status,
		row.InvitationSentAt, row.AcceptedAt, row.MessageSentAt, row.FollowupDueAt, row.ReferralReceivedAt,
		row.Notes, row.CreatedAt, row.UpdatedAt), nil
}

func (r *PgRepository) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]JobContact, int64, error) {
	rows, err := r.q.ListJobContacts(ctx, db.ListJobContactsParams{
		UserID: userID, JobID: f.JobID, ContactID: f.ContactID, Status: f.Status,
		PageLimit: p.Limit(), PageOffset: p.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountJobContacts(ctx, db.CountJobContactsParams{
		UserID: userID, JobID: f.JobID, ContactID: f.ContactID, Status: f.Status,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]JobContact, 0, len(rows))
	for _, row := range rows {
		out = append(out, build(row.ID, row.JobID, row.JobTitle, row.CompanyID, row.CompanyName,
			row.ContactID, row.ContactName, row.ContactLinkedinURL, row.Status,
			row.InvitationSentAt, row.AcceptedAt, row.MessageSentAt, row.FollowupDueAt, row.ReferralReceivedAt,
			row.Notes, row.CreatedAt, row.UpdatedAt))
	}
	return out, total, nil
}

func (r *PgRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	n, err := r.q.DeleteJobContact(ctx, db.DeleteJobContactParams{ID: id, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("job contact")
	}
	return nil
}

func (r *PgRepository) Events(ctx context.Context, userID, id uuid.UUID) ([]Event, error) {
	rows, err := r.q.ListJobContactEvents(ctx, db.ListJobContactEventsParams{UserID: userID, JobContactID: id})
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, Event{
			ID: row.ID, EventType: row.EventType, FromStatus: row.FromStatus, ToStatus: row.ToStatus,
			Metadata: json.RawMessage(row.Metadata), CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

// Transition applies a status change in its own transaction.
func (r *PgRepository) Transition(ctx context.Context, userID, id uuid.UUID, to workflow.Status, opts TransitionOptions) (JobContact, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return JobContact{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	if _, err := ApplyTransition(ctx, r.q.WithTx(tx), userID, id, to, opts); err != nil {
		return JobContact{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return JobContact{}, err
	}
	return r.Get(ctx, userID, id)
}

func (r *PgRepository) UpdateNotes(ctx context.Context, userID, id uuid.UUID, notes *string) (JobContact, error) {
	n, err := r.q.UpdateJobContactNotes(ctx, db.UpdateJobContactNotesParams{ID: id, UserID: userID, Notes: notes})
	if err != nil {
		return JobContact{}, err
	}
	if n == 0 {
		return JobContact{}, apperr.NotFound("job contact")
	}
	return r.Get(ctx, userID, id)
}

// ---- service ----

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Add(ctx context.Context, userID, jobID uuid.UUID, in AddInput) (JobContact, error) {
	errs := validate.Errors{}
	if in.ContactID == uuid.Nil {
		errs.Add("contact_id", "is required")
	}
	in.Notes = validate.TrimPtr(in.Notes)
	validate.MaxLenPtr(errs, "notes", in.Notes, 5000)
	if err := errs.Err(); err != nil {
		return JobContact{}, err
	}
	return s.repo.Create(ctx, userID, jobID, in)
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (JobContact, error) {
	return s.repo.Get(ctx, userID, id)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, f ListFilter, p pagination.Params) ([]JobContact, int64, error) {
	if f.Status != nil && !workflow.IsValid(*f.Status) {
		return nil, 0, apperr.Validation(map[string]string{"status": "unknown workflow status"})
	}
	return s.repo.List(ctx, userID, f, p)
}

func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}

// Transition validates the request and moves the job contact to a new status.
// The transition table itself is enforced inside ApplyTransition.
func (s *Service) Transition(ctx context.Context, userID, id uuid.UUID, in TransitionInput) (JobContact, error) {
	errs := validate.Errors{}
	if !workflow.IsValid(in.ToStatus) {
		errs.Add("to_status", "unknown workflow status")
	}
	in.Note = validate.TrimPtr(in.Note)
	validate.MaxLenPtr(errs, "note", in.Note, 1000)
	var at time.Time
	if in.OccurredAt != nil {
		at = *in.OccurredAt
		if at.After(time.Now().Add(time.Minute)) {
			errs.Add("occurred_at", "must not be in the future")
		}
	}
	if err := errs.Err(); err != nil {
		return JobContact{}, err
	}
	return s.repo.Transition(ctx, userID, id, workflow.Status(in.ToStatus), TransitionOptions{
		At: at, Note: in.Note, Source: workflow.SourceManual,
	})
}

func (s *Service) UpdateNotes(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (JobContact, error) {
	errs := validate.Errors{}
	in.Notes = validate.TrimPtr(in.Notes)
	validate.MaxLenPtr(errs, "notes", in.Notes, 5000)
	if err := errs.Err(); err != nil {
		return JobContact{}, err
	}
	return s.repo.UpdateNotes(ctx, userID, id, in.Notes)
}

// Events returns the audit trail; ownership is verified first so a foreign id yields 404.
func (s *Service) Events(ctx context.Context, userID, id uuid.UUID) ([]Event, error) {
	if _, err := s.repo.Get(ctx, userID, id); err != nil {
		return nil, err
	}
	return s.repo.Events(ctx, userID, id)
}

// ---- handler ----

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterJobScoped mounts POST /{id}/contacts on the /jobs router.
func (h *Handler) RegisterJobScoped(r chi.Router) {
	r.Post("/{id}/contacts", httpx.Wrap(h.add))
}

// Register mounts the /job-contacts routes.
func (h *Handler) Register(r chi.Router) {
	r.Get("/", httpx.Wrap(h.list))
	r.Get("/{id}", httpx.Wrap(h.get))
	r.Delete("/{id}", httpx.Wrap(h.delete))
	r.Get("/{id}/events", httpx.Wrap(h.events))
	r.Patch("/{id}", httpx.Wrap(h.update))
	r.Post("/{id}/transitions", httpx.Wrap(h.transition))
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in TransitionInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	jc, err := h.svc.Transition(r.Context(), userID, id, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, jc)
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
	var in UpdateInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	jc, err := h.svc.UpdateNotes(r.Context(), userID, id, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, jc)
	return nil
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	jobID, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in AddInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	jc, err := h.svc.Add(r.Context(), userID, jobID, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusCreated, jc)
	return nil
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
	jobID, err := httpx.QueryUUID(r, "job_id")
	if err != nil {
		return err
	}
	contactID, err := httpx.QueryUUID(r, "contact_id")
	if err != nil {
		return err
	}
	f := ListFilter{JobID: jobID, ContactID: contactID, Status: httpx.QueryString(r, "status")}
	items, total, err := h.svc.List(r.Context(), userID, f, p)
	if err != nil {
		return err
	}
	httpx.List(w, items, pagination.NewMeta(p, total))
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
	jc, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, jc)
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

func (h *Handler) events(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	events, err := h.svc.Events(r.Context(), userID, id)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, events)
	return nil
}
