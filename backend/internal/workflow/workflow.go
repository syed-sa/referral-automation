// Package workflow defines the referral workflow state machine.
//
// Phase 1 only needs the constants and validation; Phase 2 adds the service
// that applies transitions (conditional UPDATE + workflow_events row in one
// transaction) using CanTransition as the single source of truth.
package workflow

type Status string

const (
	StatusAdded             Status = "ADDED"
	StatusInvitationPending Status = "INVITATION_PENDING"
	StatusConnected         Status = "CONNECTED"
	StatusMessageReady      Status = "MESSAGE_READY"
	StatusMessageSent       Status = "MESSAGE_SENT"
	StatusAwaitingResponse  Status = "AWAITING_RESPONSE"
	StatusReferralReceived  Status = "REFERRAL_RECEIVED"
	StatusDeclined          Status = "DECLINED"
	StatusClosed            Status = "CLOSED"
)

// AllStatuses lists every status in workflow order.
func AllStatuses() []Status {
	return []Status{
		StatusAdded, StatusInvitationPending, StatusConnected, StatusMessageReady,
		StatusMessageSent, StatusAwaitingResponse, StatusReferralReceived,
		StatusDeclined, StatusClosed,
	}
}

func IsValid(s string) bool {
	for _, v := range AllStatuses() {
		if string(v) == s {
			return true
		}
	}
	return false
}

var transitions = map[Status][]Status{
	StatusAdded:             {StatusInvitationPending},
	StatusInvitationPending: {StatusConnected},
	StatusConnected:         {StatusMessageReady},
	StatusMessageReady:      {StatusMessageSent},
	StatusMessageSent:       {StatusAwaitingResponse},
	StatusAwaitingResponse:  {StatusReferralReceived, StatusDeclined},
	// REFERRAL_RECEIVED and DECLINED may only be closed; CLOSED is terminal.
	StatusReferralReceived: {},
	StatusDeclined:         {},
}

// CanTransition reports whether from -> to is allowed. Any status except
// CLOSED may move to CLOSED.
func CanTransition(from, to Status) bool {
	if from == StatusClosed {
		return false
	}
	if to == StatusClosed {
		return true
	}
	for _, next := range transitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// AllowedNext lists the statuses reachable from the given one (never nil).
func AllowedNext(from Status) []Status {
	out := []Status{}
	if from == StatusClosed {
		return out
	}
	out = append(out, transitions[from]...)
	return append(out, StatusClosed)
}

// EventType names rows in workflow_events.
type EventType string

const (
	EventJobContactCreated EventType = "JOB_CONTACT_CREATED"
	EventStatusChanged     EventType = "STATUS_CHANGED"
	EventMessageGenerated  EventType = "FOLLOWUP_GENERATED"
	EventEmailMatched      EventType = "GMAIL_ACCEPTANCE_MATCHED"
)

// Sources recorded in event metadata.
const (
	SourceManual   = "manual"
	SourceGmail    = "gmail"
	SourceFollowUp = "followup"
)
