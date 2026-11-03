package jobcontact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"referralautomation/internal/db"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/workflow"
)

// TransitionOptions carries optional details for a status change.
type TransitionOptions struct {
	At       time.Time      // when the real-world event happened; zero means now
	Note     *string        // free-text note stored in the event metadata
	Source   string         // workflow.Source* constant
	Metadata map[string]any // extra event metadata (never message contents or tokens)
}

// ApplyTransition is the single place where a job_contact changes status.
//
// It must be called with a *db.Queries bound to a transaction (q.WithTx(tx)),
// so the row lock, the UPDATE and the workflow event commit atomically:
//
//  1. SELECT ... FOR UPDATE serializes concurrent transitions on the row
//  2. the transition is validated against workflow.CanTransition
//  3. status and the matching timestamp column are updated
//  4. a workflow_events row is appended
//
// Everything is scoped by userID.
func ApplyTransition(ctx context.Context, q *db.Queries, userID, id uuid.UUID, to workflow.Status, opts TransitionOptions) (workflow.Status, error) {
	current, err := q.LockJobContact(ctx, db.LockJobContactParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.NotFound("job contact")
	}
	if err != nil {
		return "", err
	}
	from := workflow.Status(current)
	if !workflow.CanTransition(from, to) {
		return from, apperr.Conflict(fmt.Sprintf("cannot move from %s to %s", from, to))
	}

	at := opts.At
	if at.IsZero() {
		at = time.Now()
	}
	at = at.UTC()

	params := db.UpdateJobContactStatusParams{ID: id, UserID: userID, Status: string(to)}
	switch to {
	case workflow.StatusInvitationPending:
		params.InvitationSentAt = &at
	case workflow.StatusConnected:
		params.AcceptedAt = &at
		delay, err := q.GetFollowupDelayHours(ctx, userID)
		if err != nil {
			return from, err
		}
		due := at.Add(time.Duration(delay) * time.Hour)
		params.FollowupDueAt = &due
	case workflow.StatusMessageSent:
		params.MessageSentAt = &at
	case workflow.StatusReferralReceived:
		params.ReferralReceivedAt = &at
	}
	n, err := q.UpdateJobContactStatus(ctx, params)
	if err != nil {
		return from, err
	}
	if n == 0 {
		return from, apperr.NotFound("job contact")
	}

	meta := map[string]any{}
	for k, v := range opts.Metadata {
		meta[k] = v
	}
	if opts.Source != "" {
		meta["source"] = opts.Source
	}
	if opts.Note != nil {
		meta["note"] = *opts.Note
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return from, err
	}
	fromStr, toStr := string(from), string(to)
	if err := q.CreateWorkflowEvent(ctx, db.CreateWorkflowEventParams{
		UserID:       userID,
		JobContactID: id,
		EventType:    string(workflow.EventStatusChanged),
		FromStatus:   &fromStr,
		ToStatus:     &toStr,
		Metadata:     metaJSON,
	}); err != nil {
		return from, err
	}
	return from, nil
}
