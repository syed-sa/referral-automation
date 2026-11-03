package mailsync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"

	"referralautomation/internal/database"
	"referralautomation/internal/db"
	"referralautomation/internal/jobcontact"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/pagination"
	"referralautomation/internal/secretbox"
	"referralautomation/internal/workflow"
)

// Outcomes recorded in processed_gmail_messages.
const (
	OutcomeMatched   = "MATCHED"
	OutcomeUnmatched = "UNMATCHED"
	OutcomeIgnored   = "IGNORED"
)

const (
	oauthStateTTL   = 10 * time.Minute
	listCap         = 500
	maxSubjectChars = 300
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	FrontendURL  string
	SyncMax      int // max new messages handled per manual sync
	LookbackDays int // first-sync window
}

type Service struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	oauth     *oauth2.Config
	box       *secretbox.Box
	cfg       Config
	parser    Parser
	newSource NewSourceFunc
	http      *http.Client
}

func NewService(pool *pgxpool.Pool, q *db.Queries, cfg Config, box *secretbox.Box) *Service {
	return &Service{
		pool: pool,
		q:    q,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{gmail.GmailReadonlyScope}, // read-only, nothing broader
		},
		box:       box,
		cfg:       cfg,
		parser:    LinkedInAcceptanceParser{},
		newSource: newGoogleSource,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

// FrontendRedirect builds the URL the OAuth callback sends the browser to.
func (s *Service) FrontendRedirect(status string) string {
	return s.cfg.FrontendURL + "/settings/gmail?status=" + url.QueryEscape(status)
}

// ---- OAuth connect ----

// StartConnect creates a single-use, expiring, user-bound state and returns the
// Google consent URL.
func (s *Service) StartConnect(ctx context.Context, userID uuid.UUID) (string, error) {
	_ = s.q.DeleteExpiredOAuthStates(ctx) // opportunistic cleanup; failure is harmless

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(state))
	if err := s.q.CreateOAuthState(ctx, db.CreateOAuthStateParams{
		StateHash: hash[:], UserID: userID, ExpiresAt: time.Now().Add(oauthStateTTL),
	}); err != nil {
		return "", err
	}
	return s.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	), nil
}

// CompleteCallback validates the state, exchanges the code and stores the
// encrypted refresh token. It returns a short, non-sensitive status for the
// frontend redirect.
func (s *Service) CompleteCallback(ctx context.Context, code, state string) (string, error) {
	if code == "" || state == "" {
		return "invalid_request", errors.New("missing code or state")
	}
	hash := sha256.Sum256([]byte(state))
	userID, err := s.q.ConsumeOAuthState(ctx, hash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return "invalid_state", errors.New("unknown, used or expired state")
	}
	if err != nil {
		return "error", err
	}

	tok, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		return "exchange_failed", fmt.Errorf("exchange code: %w", err)
	}
	if tok.RefreshToken == "" {
		return "no_refresh_token", errors.New("google did not return a refresh token")
	}
	if scope, _ := tok.Extra("scope").(string); !strings.Contains(scope, gmail.GmailReadonlyScope) {
		return "scope_missing", errors.New("gmail.readonly scope was not granted")
	}

	src, err := s.newSource(ctx, s.oauth.TokenSource(ctx, tok))
	if err != nil {
		return "error", err
	}
	address, err := src.ProfileEmail(ctx)
	if err != nil {
		return "error", fmt.Errorf("read gmail profile: %w", err)
	}
	enc, err := s.box.Seal([]byte(tok.RefreshToken), userID[:])
	if err != nil {
		return "error", err
	}
	if _, err := s.q.UpsertEmailAccount(ctx, db.UpsertEmailAccountParams{
		UserID: userID, EmailAddress: address, EncryptedRefreshToken: enc, Scope: gmail.GmailReadonlyScope,
	}); err != nil {
		return "error", err
	}
	return "connected", nil
}

// ---- status / disconnect ----

type SyncRecord struct {
	ID           uuid.UUID  `json:"id"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	MessagesSeen int32      `json:"messages_seen"`
	Matched      int32      `json:"matched"`
	Unmatched    int32      `json:"unmatched"`
	Ignored      int32      `json:"ignored"`
	Error        *string    `json:"error"`
}

type AccountStatus struct {
	Enabled      bool         `json:"enabled"`
	Connected    bool         `json:"connected"`
	EmailAddress *string      `json:"email_address"`
	Status       *string      `json:"status"`
	LastSyncedAt *time.Time   `json:"last_synced_at"`
	RecentSyncs  []SyncRecord `json:"recent_syncs"`
}

func (s *Service) Status(ctx context.Context, userID uuid.UUID) (AccountStatus, error) {
	out := AccountStatus{Enabled: true, RecentSyncs: []SyncRecord{}}
	acct, err := s.q.GetEmailAccount(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Connected = true
	out.EmailAddress = &acct.EmailAddress
	out.Status = &acct.Status
	out.LastSyncedAt = acct.LastSyncedAt

	rows, err := s.q.ListSyncHistory(ctx, db.ListSyncHistoryParams{UserID: userID, RowLimit: 5})
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		out.RecentSyncs = append(out.RecentSyncs, SyncRecord{
			ID: r.ID, Status: r.Status, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
			MessagesSeen: r.MessagesSeen, Matched: r.Matched, Unmatched: r.Unmatched,
			Ignored: r.Ignored, Error: r.Error,
		})
	}
	return out, nil
}

// Disconnect deletes the stored account (and, via cascade, its sync history,
// processed-message ledger and review queue) and revokes the token at Google
// on a best-effort basis.
func (s *Service) Disconnect(ctx context.Context, userID uuid.UUID) error {
	acct, err := s.q.GetEmailAccount(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("gmail connection")
	}
	if err != nil {
		return err
	}
	if rt, err := s.box.Open(acct.EncryptedRefreshToken, userID[:]); err == nil {
		s.revoke(ctx, string(rt))
	}
	n, err := s.q.DeleteEmailAccount(ctx, db.DeleteEmailAccountParams{ID: acct.ID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("gmail connection")
	}
	return nil
}

func (s *Service) revoke(ctx context.Context, token string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/revoke?token="+url.QueryEscape(token), nil)
	if err != nil {
		return
	}
	resp, err := s.http.Do(req)
	if err != nil {
		slog.Warn("token revoke failed") // never log the token or the URL
		return
	}
	resp.Body.Close()
}

// ---- sync ----

type SyncResult struct {
	SyncID    uuid.UUID `json:"sync_id"`
	Processed int       `json:"processed"`
	Matched   int       `json:"matched"`
	Unmatched int       `json:"unmatched"`
	Ignored   int       `json:"ignored"`
	Failed    int       `json:"failed"`
	HasMore   bool      `json:"has_more"`
}

func (s *Service) searchQuery(last *time.Time) string {
	from := time.Now().AddDate(0, 0, -s.cfg.LookbackDays)
	if last != nil {
		from = last.AddDate(0, 0, -2) // overlap; the processed ledger makes re-reading harmless
	}
	return fmt.Sprintf("from:linkedin.com after:%d", from.Unix())
}

// Sync is the manual sync. It is idempotent (processed ledger keyed by Gmail
// message id) and concurrency-safe (one RUNNING row per account).
func (s *Service) Sync(ctx context.Context, userID uuid.UUID) (res SyncResult, err error) {
	acct, err := s.q.GetEmailAccount(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, apperr.New(http.StatusConflict, "gmail_not_connected", "connect a Gmail account first")
	}
	if err != nil {
		return res, err
	}
	if acct.Status != "ACTIVE" {
		return res, reauthErr()
	}

	if err := s.q.FailStaleSyncs(ctx, userID); err != nil {
		return res, err
	}
	syncID, err := s.q.StartSync(ctx, db.StartSyncParams{UserID: userID, EmailAccountID: acct.ID})
	if database.IsUniqueViolation(err) {
		return res, apperr.Conflict("a sync is already running")
	}
	if err != nil {
		return res, err
	}
	res.SyncID = syncID
	startedAt := time.Now().UTC()

	defer func() {
		// Detached context: the bookkeeping must be written even if the request was cancelled.
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		status := "SUCCEEDED"
		var msg *string
		switch {
		case err != nil:
			status = "FAILED"
			m := truncate(err.Error(), 300)
			msg = &m
		case res.Failed > 0:
			m := fmt.Sprintf("%d message(s) failed and will be retried on the next sync", res.Failed)
			msg = &m
		}
		if ferr := s.q.FinishSync(fctx, db.FinishSyncParams{
			ID: syncID, UserID: userID, Status: status, MessagesSeen: int32(res.Processed),
			Matched: int32(res.Matched), Unmatched: int32(res.Unmatched), Ignored: int32(res.Ignored), Error: msg,
		}); ferr != nil {
			slog.Error("finish sync failed", "error", ferr)
		}
		if err == nil {
			if serr := s.q.SetEmailAccountSynced(fctx, db.SetEmailAccountSyncedParams{
				ID: acct.ID, UserID: userID, LastSyncedAt: &startedAt,
			}); serr != nil {
				slog.Error("update last_synced_at failed", "error", serr)
			}
		}
	}()

	refresh, err := s.box.Open(acct.EncryptedRefreshToken, userID[:])
	if err != nil {
		return res, fmt.Errorf("decrypt refresh token: %w", err)
	}
	src, err := s.newSource(ctx, s.oauth.TokenSource(ctx, &oauth2.Token{RefreshToken: string(refresh)}))
	if err != nil {
		return res, err
	}

	ids, more, err := src.ListIDs(ctx, s.searchQuery(acct.LastSyncedAt), listCap)
	if err != nil {
		return res, s.mapGoogleErr(ctx, userID, acct.ID, err)
	}
	already, err := s.q.FilterProcessedMessages(ctx, db.FilterProcessedMessagesParams{UserID: userID, MessageIds: ids})
	if err != nil {
		return res, err
	}
	done := make(map[string]struct{}, len(already))
	for _, id := range already {
		done[id] = struct{}{}
	}
	fresh := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := done[id]; !ok {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) > s.cfg.SyncMax {
		fresh, more = fresh[:s.cfg.SyncMax], true
	}
	res.HasMore = more

	rows, err := s.q.ListPendingInvitationTargets(ctx, userID)
	if err != nil {
		return res, err
	}
	targets := make([]Target, 0, len(rows))
	for _, r := range rows {
		targets = append(targets, Target{ContactID: r.ContactID, FullName: r.FullName, LinkedInURL: r.LinkedinURL})
	}

	for _, id := range fresh {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		msg, gerr := src.Get(ctx, id)
		if gerr != nil {
			if isInvalidGrant(gerr) {
				return res, s.mapGoogleErr(ctx, userID, acct.ID, gerr)
			}
			res.Failed++
			slog.Warn("fetch gmail message failed", "error", gerr)
			continue
		}
		outcome, perr := s.processMessage(ctx, userID, acct.ID, msg, targets)
		if perr != nil {
			res.Failed++
			slog.Warn("process gmail message failed", "error", perr)
			continue
		}
		switch outcome {
		case OutcomeMatched:
			res.Matched++
		case OutcomeUnmatched:
			res.Unmatched++
		case OutcomeIgnored:
			res.Ignored++
		default:
			continue // another sync handled it first
		}
		res.Processed++
	}
	return res, nil
}

// processMessage handles one message in a single transaction: ledger row,
// status transitions and review-queue row commit together or not at all.
// It returns "" when the message had already been processed.
func (s *Service) processMessage(ctx context.Context, userID, accountID uuid.UUID, msg Message, targets []Target) (string, error) {
	parsed, isAcceptance := s.parser.Parse(msg)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := s.q.WithTx(tx)

	outcome := OutcomeIgnored
	var (
		pending []uuid.UUID
		reason  string
	)
	if isAcceptance {
		outcome = OutcomeUnmatched
		contactID, why, ok := matchContact(parsed, targets)
		if ok {
			pending, err = q.ListPendingInvitationsForContact(ctx, db.ListPendingInvitationsForContactParams{
				UserID: userID, ContactID: contactID,
			})
			if err != nil {
				return "", err
			}
			if len(pending) > 0 {
				outcome = OutcomeMatched
			} else {
				outcome = OutcomeIgnored // already handled elsewhere (e.g. marked manually)
			}
		} else {
			reason = why
		}
	}

	n, err := q.InsertProcessedMessage(ctx, db.InsertProcessedMessageParams{
		UserID: userID, EmailAccountID: accountID, GmailMessageID: msg.ID, Outcome: outcome,
	})
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}

	switch outcome {
	case OutcomeMatched:
		for _, jcID := range pending {
			if _, err := jobcontact.ApplyTransition(ctx, q, userID, jcID, workflow.StatusConnected, jobcontact.TransitionOptions{
				At:       msg.ReceivedAt,
				Source:   workflow.SourceGmail,
				Metadata: map[string]any{"gmail_message_id": msg.ID},
			}); err != nil {
				return "", err
			}
		}
	case OutcomeUnmatched:
		if err := q.InsertUnmatchedEmail(ctx, db.InsertUnmatchedEmailParams{
			UserID: userID, EmailAccountID: accountID, GmailMessageID: msg.ID,
			FromAddress: truncate(msg.From, 320), Subject: truncate(msg.Subject, maxSubjectChars),
			ReceivedAt: msg.ReceivedAt, ExtractedName: nonEmpty(parsed.PersonName),
			ExtractedProfileURL: nonEmpty(parsed.ProfileURL), Reason: reason,
		}); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return outcome, nil
}

func (s *Service) mapGoogleErr(ctx context.Context, userID, accountID uuid.UUID, err error) error {
	if isInvalidGrant(err) {
		if serr := s.q.SetEmailAccountStatus(context.WithoutCancel(ctx), db.SetEmailAccountStatusParams{
			ID: accountID, UserID: userID, Status: "NEEDS_REAUTH",
		}); serr != nil {
			slog.Error("mark account NEEDS_REAUTH failed", "error", serr)
		}
		return reauthErr()
	}
	e := apperr.New(http.StatusBadGateway, "gmail_error", "Gmail request failed")
	e.Err = err
	return e
}

func reauthErr() error {
	return apperr.New(http.StatusConflict, "gmail_reauth_required", "Gmail access was revoked or expired; reconnect your account")
}

func isInvalidGrant(err error) bool {
	var re *oauth2.RetrieveError
	return errors.As(err, &re) && re.ErrorCode == "invalid_grant"
}

// ---- unmatched review queue ----

type UnmatchedEmail struct {
	ID                  uuid.UUID  `json:"id"`
	FromAddress         string     `json:"from_address"`
	Subject             string     `json:"subject"`
	ReceivedAt          time.Time  `json:"received_at"`
	ExtractedName       *string    `json:"extracted_name"`
	ExtractedProfileURL *string    `json:"extracted_profile_url"`
	Reason              string     `json:"reason"`
	Status              string     `json:"status"`
	ResolvedAt          *time.Time `json:"resolved_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

func (s *Service) ListUnmatched(ctx context.Context, userID uuid.UUID, status *string, p pagination.Params) ([]UnmatchedEmail, int64, error) {
	if status != nil && *status != "PENDING" && *status != "RESOLVED" && *status != "DISMISSED" {
		return nil, 0, apperr.Validation(map[string]string{"status": "must be PENDING, RESOLVED or DISMISSED"})
	}
	rows, err := s.q.ListUnmatchedEmails(ctx, db.ListUnmatchedEmailsParams{
		UserID: userID, Status: status, PageLimit: p.Limit(), PageOffset: p.Offset(),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountUnmatchedEmails(ctx, db.CountUnmatchedEmailsParams{UserID: userID, Status: status})
	if err != nil {
		return nil, 0, err
	}
	out := make([]UnmatchedEmail, 0, len(rows))
	for _, r := range rows {
		out = append(out, UnmatchedEmail{
			ID: r.ID, FromAddress: r.FromAddress, Subject: r.Subject, ReceivedAt: r.ReceivedAt,
			ExtractedName: r.ExtractedName, ExtractedProfileURL: r.ExtractedProfileURL, Reason: r.Reason,
			Status: r.Status, ResolvedAt: r.ResolvedAt, CreatedAt: r.CreatedAt,
		})
	}
	return out, total, nil
}

// Resolve links a queued email to a contact the user picked and moves that
// contact's pending invitations to CONNECTED.
func (s *Service) Resolve(ctx context.Context, userID, emailID, contactID uuid.UUID) error {
	if contactID == uuid.Nil {
		return apperr.Validation(map[string]string{"contact_id": "is required"})
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := s.q.WithTx(tx)

	item, err := q.GetUnmatchedEmail(ctx, db.GetUnmatchedEmailParams{ID: emailID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("unmatched email")
	}
	if err != nil {
		return err
	}
	if item.Status != "PENDING" {
		return apperr.Conflict("this email was already handled")
	}
	if _, err := q.GetContact(ctx, db.GetContactParams{ID: contactID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
		return apperr.Validation(map[string]string{"contact_id": "contact not found"})
	} else if err != nil {
		return err
	}
	pending, err := q.ListPendingInvitationsForContact(ctx, db.ListPendingInvitationsForContactParams{
		UserID: userID, ContactID: contactID,
	})
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return apperr.Conflict("this contact has no pending invitation")
	}
	for _, jcID := range pending {
		if _, err := jobcontact.ApplyTransition(ctx, q, userID, jcID, workflow.StatusConnected, jobcontact.TransitionOptions{
			At:       item.ReceivedAt,
			Source:   workflow.SourceGmail,
			Metadata: map[string]any{"resolved_from_review_queue": true},
		}); err != nil {
			return err
		}
	}
	n, err := q.ResolveUnmatchedEmail(ctx, db.ResolveUnmatchedEmailParams{ID: emailID, UserID: userID, Status: "RESOLVED"})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.Conflict("this email was already handled")
	}
	return tx.Commit(ctx)
}

func (s *Service) Dismiss(ctx context.Context, userID, emailID uuid.UUID) error {
	n, err := s.q.ResolveUnmatchedEmail(ctx, db.ResolveUnmatchedEmailParams{ID: emailID, UserID: userID, Status: "DISMISSED"})
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.q.GetUnmatchedEmail(ctx, db.GetUnmatchedEmailParams{ID: emailID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("unmatched email")
		}
		return apperr.Conflict("this email was already handled")
	}
	return nil
}

// ---- helpers ----

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func nonEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
