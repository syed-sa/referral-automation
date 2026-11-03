package followup

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"referralautomation/internal/database"
	"referralautomation/internal/db"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/validate"
)

// ---- templates ----

type Template struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemplateInput is the create/update body. Supported placeholders:
// {{contact_name}} {{contact_first_name}} {{company}} {{job_title}} {{job_url}} {{sender_name}}
type TemplateInput struct {
	Name      string `json:"name"`
	Body      string `json:"body"`
	IsDefault bool   `json:"is_default"`
}

func normalizeTemplate(in TemplateInput) (TemplateInput, error) {
	errs := validate.Errors{}
	in.Name = validate.Trim(in.Name)
	validate.Required(errs, "name", in.Name)
	validate.MaxLen(errs, "name", in.Name, 100)
	in.Body = strings.TrimSpace(in.Body)
	validate.Required(errs, "body", in.Body)
	validate.MaxLen(errs, "body", in.Body, 4000)
	if bad := unknownPlaceholders(in.Body); len(bad) > 0 {
		errs.Add("body", "unknown placeholder(s): "+strings.Join(bad, ", "))
	}
	return in, errs.Err()
}

func toTemplate(id uuid.UUID, name, kind, body string, isDefault bool, created, updated time.Time) Template {
	return Template{ID: id, Name: name, Kind: kind, Body: body, IsDefault: isDefault, CreatedAt: created, UpdatedAt: updated}
}

func mapTemplateErr(err error) error {
	if database.IsUniqueViolation(err) {
		return apperr.Conflict("a template with this name already exists")
	}
	return err
}

func (s *Service) CreateTemplate(ctx context.Context, userID uuid.UUID, in TemplateInput) (Template, error) {
	in, err := normalizeTemplate(in)
	if err != nil {
		return Template{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := s.q.WithTx(tx)

	if in.IsDefault {
		if err := q.ClearDefaultTemplates(ctx, userID); err != nil {
			return Template{}, err
		}
	}
	id, err := q.CreateTemplate(ctx, db.CreateTemplateParams{UserID: userID, Name: in.Name, Body: in.Body, IsDefault: in.IsDefault})
	if err != nil {
		return Template{}, mapTemplateErr(err)
	}
	row, err := q.GetTemplate(ctx, db.GetTemplateParams{ID: id, UserID: userID})
	if err != nil {
		return Template{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return toTemplate(row.ID, row.Name, row.Kind, row.Body, row.IsDefault, row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) GetTemplate(ctx context.Context, userID, id uuid.UUID) (Template, error) {
	row, err := s.q.GetTemplate(ctx, db.GetTemplateParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Template{}, apperr.NotFound("template")
	}
	if err != nil {
		return Template{}, err
	}
	return toTemplate(row.ID, row.Name, row.Kind, row.Body, row.IsDefault, row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) ListTemplates(ctx context.Context, userID uuid.UUID) ([]Template, error) {
	rows, err := s.q.ListTemplates(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTemplate(r.ID, r.Name, r.Kind, r.Body, r.IsDefault, r.CreatedAt, r.UpdatedAt))
	}
	return out, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, userID, id uuid.UUID, in TemplateInput) (Template, error) {
	in, err := normalizeTemplate(in)
	if err != nil {
		return Template{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := s.q.WithTx(tx)

	if in.IsDefault {
		if err := q.ClearDefaultTemplates(ctx, userID); err != nil {
			return Template{}, err
		}
	}
	n, err := q.UpdateTemplate(ctx, db.UpdateTemplateParams{
		ID: id, UserID: userID, Name: in.Name, Body: in.Body, IsDefault: in.IsDefault,
	})
	if err != nil {
		return Template{}, mapTemplateErr(err)
	}
	if n == 0 {
		return Template{}, apperr.NotFound("template")
	}
	row, err := q.GetTemplate(ctx, db.GetTemplateParams{ID: id, UserID: userID})
	if err != nil {
		return Template{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return toTemplate(row.ID, row.Name, row.Kind, row.Body, row.IsDefault, row.CreatedAt, row.UpdatedAt), nil
}

// DeleteTemplate keeps messages already generated from it (template_id becomes NULL).
func (s *Service) DeleteTemplate(ctx context.Context, userID, id uuid.UUID) error {
	n, err := s.q.DeleteTemplate(ctx, db.DeleteTemplateParams{ID: id, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("template")
	}
	return nil
}

// ---- settings ----

type Settings struct {
	FollowUpDelayHours int32      `json:"followup_delay_hours"`
	SenderName         *string    `json:"sender_name"`
	UpdatedAt          *time.Time `json:"updated_at"`
}

type SettingsInput struct {
	FollowUpDelayHours int32   `json:"followup_delay_hours"`
	SenderName         *string `json:"sender_name"`
}

const defaultFollowUpDelayHours = 24

func (s *Service) GetSettings(ctx context.Context, userID uuid.UUID) (Settings, error) {
	row, err := s.q.GetSettings(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{FollowUpDelayHours: defaultFollowUpDelayHours}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return Settings{FollowUpDelayHours: row.FollowupDelayHours, SenderName: row.SenderName, UpdatedAt: &row.UpdatedAt}, nil
}

func (s *Service) UpdateSettings(ctx context.Context, userID uuid.UUID, in SettingsInput) (Settings, error) {
	errs := validate.Errors{}
	if in.FollowUpDelayHours < 1 || in.FollowUpDelayHours > 720 {
		errs.Add("followup_delay_hours", "must be between 1 and 720")
	}
	in.SenderName = validate.TrimPtr(in.SenderName)
	validate.MaxLenPtr(errs, "sender_name", in.SenderName, 100)
	if err := errs.Err(); err != nil {
		return Settings{}, err
	}
	if err := s.q.UpsertSettings(ctx, db.UpsertSettingsParams{
		UserID: userID, FollowupDelayHours: in.FollowUpDelayHours, SenderName: in.SenderName,
	}); err != nil {
		return Settings{}, err
	}
	return s.GetSettings(ctx, userID)
}
