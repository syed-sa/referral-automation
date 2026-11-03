// Package followup generates follow-up message drafts from templates (no AI),
// tracks them, and manages templates and per-user settings.
package followup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"referralautomation/internal/db"
	"referralautomation/internal/jobcontact"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/pagination"
	"referralautomation/internal/workflow"
)

// builtinTemplate is used when the user has no default template.
const builtinTemplate = `Hi {{contact_first_name}},

Thanks for connecting! I'm interested in the {{job_title}} role at {{company}} and would really appreciate it if you could refer me or point me to the right person. Happy to share my resume and any details that would help.

Thanks again,
{{sender_name}}`

// allowedPlaceholders are the only variables a template may use.
var allowedPlaceholders = map[string]struct{}{
	"contact_name": {}, "contact_first_name": {}, "company": {},
	"job_title": {}, "job_url": {}, "sender_name": {},
}

var placeholderRe = regexp.MustCompile(`\{\{\s*([a-zA-Z_]+)\s*\}\}`)

// unknownPlaceholders returns the placeholders in body that are not supported.
func unknownPlaceholders(body string) []string {
	var bad []string
	for _, m := range placeholderRe.FindAllStringSubmatch(body, -1) {
		if _, ok := allowedPlaceholders[strings.ToLower(m[1])]; !ok {
			bad = append(bad, m[1])
		}
	}
	return bad
}

func render(body string, vars map[string]string) string {
	out := placeholderRe.ReplaceAllStringFunc(body, func(m string) string {
		name := strings.ToLower(placeholderRe.FindStringSubmatch(m)[1])
		if v, ok := vars[name]; ok {
			return v
		}
		return m
	})
	return strings.TrimSpace(out)
}

func firstName(full string) string {
	if f := strings.Fields(full); len(f) > 0 {
		return f[0]
	}
	return ""
}

// ---- models ----

type Message struct {
	ID           uuid.UUID  `json:"id"`
	JobContactID uuid.UUID  `json:"job_contact_id"`
	TemplateID   *uuid.UUID `json:"template_id"`
	Kind         string     `json:"kind"`
	Body         string     `json:"body"`
	Status       string     `json:"status"`
	GeneratedAt  time.Time  `json:"generated_at"`
	SentAt       *time.Time `json:"sent_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func toMessage(id, jobContactID uuid.UUID, templateID *uuid.UUID, kind, body, status string,
	generated time.Time, sent *time.Time, created, updated time.Time) Message {
	return Message{ID: id, JobContactID: jobContactID, TemplateID: templateID, Kind: kind, Body: body,
		Status: status, GeneratedAt: generated, SentAt: sent, CreatedAt: created, UpdatedAt: updated}
}

type Due struct {
	JobContactID  uuid.UUID  `json:"job_contact_id"`
	JobID         uuid.UUID  `json:"job_id"`
	JobTitle      string     `json:"job_title"`
	CompanyName   string     `json:"company_name"`
	ContactID     uuid.UUID  `json:"contact_id"`
	ContactName   string     `json:"contact_name"`
	AcceptedAt    *time.Time `json:"accepted_at"`
	FollowUpDueAt *time.Time `json:"followup_due_at"`
}

// GenerateInput is the optional body of POST /job-contacts/{id}/followup.
type GenerateInput struct {
	TemplateID *uuid.UUID `json:"template_id"` // use this template instead of the default
	Regenerate bool       `json:"regenerate"`  // replace the existing draft
}

// ---- service ----

type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, q *db.Queries) *Service {
	return &Service{pool: pool, q: q, now: time.Now}
}

// Generate creates (or returns) the follow-up draft for a job contact.
//
// Idempotency and concurrency: the job_contacts row is locked FOR UPDATE for
// the whole operation, so two simultaneous calls serialize; the second sees the
// first one's draft and returns it. The partial unique index on
// messages(user_id, job_contact_id, kind) WHERE status = 'DRAFT' is the
// database-level backstop. The returned bool is true when a new draft was made.
//
// Eligibility: the job contact must be CONNECTED and at least the user's
// follow-up delay (default 24h) must have passed since acceptance. A contact
// already in MESSAGE_READY returns its existing draft, or a regenerated one.
func (s *Service) Generate(ctx context.Context, userID, jobContactID uuid.UUID, in GenerateInput) (Message, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := s.q.WithTx(tx)

	status, err := q.LockJobContact(ctx, db.LockJobContactParams{ID: jobContactID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, apperr.NotFound("job contact")
	}
	if err != nil {
		return Message{}, false, err
	}
	fc, err := q.GetFollowupContext(ctx, db.GetFollowupContextParams{ID: jobContactID, UserID: userID})
	if err != nil {
		return Message{}, false, err
	}
	delay, err := q.GetFollowupDelayHours(ctx, userID)
	if err != nil {
		return Message{}, false, err
	}

	switch workflow.Status(status) {
	case workflow.StatusConnected:
		if fc.AcceptedAt == nil {
			return Message{}, false, apperr.New(http.StatusConflict, "followup_not_eligible", "connection has no acceptance time yet")
		}
		eligibleAt := fc.AcceptedAt.Add(time.Duration(delay) * time.Hour)
		if s.now().Before(eligibleAt) {
			return Message{}, false, apperr.New(http.StatusConflict, "followup_not_eligible",
				"follow-up becomes available at "+eligibleAt.UTC().Format(time.RFC3339))
		}
	case workflow.StatusMessageReady:
		// existing draft or regeneration, handled below
	default:
		return Message{}, false, apperr.New(http.StatusConflict, "followup_not_eligible",
			fmt.Sprintf("follow-ups can be generated in CONNECTED or MESSAGE_READY, not %s", status))
	}

	existing, err := q.GetDraftMessage(ctx, db.GetDraftMessageParams{UserID: userID, JobContactID: jobContactID})
	hasDraft := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, err
	}
	if hasDraft && !in.Regenerate && in.TemplateID == nil {
		return toMessage(existing.ID, existing.JobContactID, existing.TemplateID, existing.Kind, existing.Body,
			existing.Status, existing.GeneratedAt, existing.SentAt, existing.CreatedAt, existing.UpdatedAt), false, nil
	}

	// Pick the template: explicit, then the user's default, then the built-in text.
	body, templateID, err := s.templateBody(ctx, q, userID, in.TemplateID)
	if err != nil {
		return Message{}, false, err
	}
	var senderName string
	if st, err := q.GetSettings(ctx, userID); err == nil && st.SenderName != nil {
		senderName = *st.SenderName
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, err
	}
	jobURL := ""
	if fc.JobURL != nil {
		jobURL = *fc.JobURL
	}
	text := render(body, map[string]string{
		"contact_name":       fc.ContactName,
		"contact_first_name": firstName(fc.ContactName),
		"company":            fc.CompanyName,
		"job_title":          fc.JobTitle,
		"job_url":            jobURL,
		"sender_name":        senderName,
	})

	if hasDraft {
		if _, err := q.SupersedeDraftMessages(ctx, db.SupersedeDraftMessagesParams{UserID: userID, JobContactID: jobContactID}); err != nil {
			return Message{}, false, err
		}
	}
	msgID, err := q.CreateMessage(ctx, db.CreateMessageParams{
		UserID: userID, JobContactID: jobContactID, TemplateID: templateID, Body: text,
	})
	if err != nil {
		return Message{}, false, err
	}

	if workflow.Status(status) == workflow.StatusConnected {
		if _, err := jobcontact.ApplyTransition(ctx, q, userID, jobContactID, workflow.StatusMessageReady, jobcontact.TransitionOptions{
			At: s.now(), Source: workflow.SourceFollowUp, Metadata: map[string]any{"message_id": msgID.String()},
		}); err != nil {
			return Message{}, false, err
		}
	}
	meta, err := json.Marshal(map[string]any{"message_id": msgID.String(), "regenerated": hasDraft})
	if err != nil {
		return Message{}, false, err
	}
	if err := q.CreateWorkflowEvent(ctx, db.CreateWorkflowEventParams{
		UserID: userID, JobContactID: jobContactID, EventType: string(workflow.EventMessageGenerated), Metadata: meta,
	}); err != nil {
		return Message{}, false, err
	}

	row, err := q.GetMessage(ctx, db.GetMessageParams{ID: msgID, UserID: userID})
	if err != nil {
		return Message{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Message{}, false, err
	}
	return toMessage(row.ID, row.JobContactID, row.TemplateID, row.Kind, row.Body, row.Status,
		row.GeneratedAt, row.SentAt, row.CreatedAt, row.UpdatedAt), true, nil
}

func (s *Service) templateBody(ctx context.Context, q *db.Queries, userID uuid.UUID, explicit *uuid.UUID) (string, *uuid.UUID, error) {
	if explicit != nil {
		t, err := q.GetTemplate(ctx, db.GetTemplateParams{ID: *explicit, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, apperr.Validation(map[string]string{"template_id": "template not found"})
		}
		if err != nil {
			return "", nil, err
		}
		return t.Body, &t.ID, nil
	}
	t, err := q.GetDefaultTemplate(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return builtinTemplate, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return t.Body, &t.ID, nil
}

// MarkSent records that the user sent the draft themselves (the app never
// sends anything): the draft becomes SENT and the job contact moves
// MESSAGE_READY -> MESSAGE_SENT -> AWAITING_RESPONSE atomically.
func (s *Service) MarkSent(ctx context.Context, userID, jobContactID uuid.UUID) (Message, workflow.Status, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Message{}, "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := s.q.WithTx(tx)

	status, err := q.LockJobContact(ctx, db.LockJobContactParams{ID: jobContactID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, "", apperr.NotFound("job contact")
	}
	if err != nil {
		return Message{}, "", err
	}
	if workflow.Status(status) != workflow.StatusMessageReady {
		return Message{}, "", apperr.Conflict(fmt.Sprintf("job contact is %s; expected MESSAGE_READY", status))
	}
	draft, err := q.GetDraftMessage(ctx, db.GetDraftMessageParams{UserID: userID, JobContactID: jobContactID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, "", apperr.Conflict("there is no draft to mark as sent")
	}
	if err != nil {
		return Message{}, "", err
	}
	if n, err := q.MarkMessageSent(ctx, db.MarkMessageSentParams{ID: draft.ID, UserID: userID}); err != nil {
		return Message{}, "", err
	} else if n == 0 {
		return Message{}, "", apperr.Conflict("draft was already handled")
	}
	now := s.now()
	meta := map[string]any{"message_id": draft.ID.String()}
	for _, to := range []workflow.Status{workflow.StatusMessageSent, workflow.StatusAwaitingResponse} {
		if _, err := jobcontact.ApplyTransition(ctx, q, userID, jobContactID, to, jobcontact.TransitionOptions{
			At: now, Source: workflow.SourceFollowUp, Metadata: meta,
		}); err != nil {
			return Message{}, "", err
		}
	}
	row, err := q.GetMessage(ctx, db.GetMessageParams{ID: draft.ID, UserID: userID})
	if err != nil {
		return Message{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return Message{}, "", err
	}
	return toMessage(row.ID, row.JobContactID, row.TemplateID, row.Kind, row.Body, row.Status,
		row.GeneratedAt, row.SentAt, row.CreatedAt, row.UpdatedAt), workflow.StatusAwaitingResponse, nil
}

// ListMessages returns the messages of a job contact (404 if it is not the caller's).
func (s *Service) ListMessages(ctx context.Context, userID, jobContactID uuid.UUID) ([]Message, error) {
	if _, err := s.q.GetJobContact(ctx, db.GetJobContactParams{ID: jobContactID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("job contact")
	} else if err != nil {
		return nil, err
	}
	rows, err := s.q.ListMessagesForJobContact(ctx, db.ListMessagesForJobContactParams{UserID: userID, JobContactID: jobContactID})
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMessage(r.ID, r.JobContactID, r.TemplateID, r.Kind, r.Body, r.Status,
			r.GeneratedAt, r.SentAt, r.CreatedAt, r.UpdatedAt))
	}
	return out, nil
}

// EditDraft lets the user tweak a generated draft before sending it.
func (s *Service) EditDraft(ctx context.Context, userID, messageID uuid.UUID, body string) (Message, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 4000 {
		return Message{}, apperr.Validation(map[string]string{"body": "is required and must be at most 4000 characters"})
	}
	n, err := s.q.UpdateDraftMessageBody(ctx, db.UpdateDraftMessageBodyParams{ID: messageID, UserID: userID, Body: body})
	if err != nil {
		return Message{}, err
	}
	row, gerr := s.q.GetMessage(ctx, db.GetMessageParams{ID: messageID, UserID: userID})
	if errors.Is(gerr, pgx.ErrNoRows) {
		return Message{}, apperr.NotFound("message")
	}
	if gerr != nil {
		return Message{}, gerr
	}
	if n == 0 {
		return Message{}, apperr.Conflict("only draft messages can be edited")
	}
	return toMessage(row.ID, row.JobContactID, row.TemplateID, row.Kind, row.Body, row.Status,
		row.GeneratedAt, row.SentAt, row.CreatedAt, row.UpdatedAt), nil
}

// ListDue lists CONNECTED job contacts whose follow-up is due.
func (s *Service) ListDue(ctx context.Context, userID uuid.UUID, p pagination.Params) ([]Due, int64, error) {
	delay, err := s.q.GetFollowupDelayHours(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListFollowupsDue(ctx, db.ListFollowupsDueParams{
		UserID: userID, DelayHours: delay, PageLimit: p.Limit(), PageOffset: p.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountFollowupsDue(ctx, db.CountFollowupsDueParams{UserID: userID, DelayHours: delay})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Due, 0, len(rows))
	for _, r := range rows {
		out = append(out, Due{
			JobContactID: r.ID, JobID: r.JobID, JobTitle: r.JobTitle, CompanyName: r.CompanyName,
			ContactID: r.ContactID, ContactName: r.ContactName, AcceptedAt: r.AcceptedAt, FollowUpDueAt: r.FollowupDueAt,
		})
	}
	return out, total, nil
}
