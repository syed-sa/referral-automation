// Package dashboard serves the user's summary counts.
package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"referralautomation/internal/auth"
	"referralautomation/internal/db"
	"referralautomation/internal/platform/httpx"
	"referralautomation/internal/workflow"
)

type RecentEvent struct {
	ID           uuid.UUID `json:"id"`
	JobContactID uuid.UUID `json:"job_contact_id"`
	ContactName  string    `json:"contact_name"`
	EventType    string    `json:"event_type"`
	FromStatus   *string   `json:"from_status"`
	ToStatus     *string   `json:"to_status"`
	CreatedAt    time.Time `json:"created_at"`
}

type Summary struct {
	Companies           int64            `json:"companies"`
	ActiveJobs          int64            `json:"active_jobs"`
	Contacts            int64            `json:"contacts"`
	FollowUpsDue        int64            `json:"followups_due"`
	JobContactsByStatus map[string]int64 `json:"job_contacts_by_status"`
	RecentActivity      []RecentEvent    `json:"recent_activity"`
}

type Service struct{ q *db.Queries }

func NewService(q *db.Queries) *Service { return &Service{q: q} }

func (s *Service) Summary(ctx context.Context, userID uuid.UUID) (Summary, error) {
	counts, err := s.q.GetDashboardCounts(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	byStatus, err := s.q.CountJobContactsByStatus(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	events, err := s.q.ListRecentWorkflowEvents(ctx, db.ListRecentWorkflowEventsParams{UserID: userID, RowLimit: 10})
	if err != nil {
		return Summary{}, err
	}

	sum := Summary{
		Companies:           counts.Companies,
		ActiveJobs:          counts.ActiveJobs,
		Contacts:            counts.Contacts,
		FollowUpsDue:        counts.FollowupsDue,
		JobContactsByStatus: make(map[string]int64, len(workflow.AllStatuses())),
		RecentActivity:      make([]RecentEvent, 0, len(events)),
	}
	for _, st := range workflow.AllStatuses() {
		sum.JobContactsByStatus[string(st)] = 0
	}
	for _, row := range byStatus {
		sum.JobContactsByStatus[row.Status] = row.Total
	}
	for _, e := range events {
		sum.RecentActivity = append(sum.RecentActivity, RecentEvent{
			ID: e.ID, JobContactID: e.JobContactID, ContactName: e.ContactName, EventType: e.EventType,
			FromStatus: e.FromStatus, ToStatus: e.ToStatus, CreatedAt: e.CreatedAt,
		})
	}
	return sum, nil
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r chi.Router) {
	r.Get("/dashboard", httpx.Wrap(h.summary))
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	s, err := h.svc.Summary(r.Context(), userID)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, s)
	return nil
}
