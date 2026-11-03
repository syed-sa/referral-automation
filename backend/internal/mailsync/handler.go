package mailsync

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"referralautomation/internal/auth"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/httpx"
	"referralautomation/internal/platform/pagination"
)

// Handler serves the Gmail endpoints. svc is nil when Gmail is not configured;
// every endpoint except status then answers 503.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register mounts the authenticated routes under /gmail.
func (h *Handler) Register(r chi.Router) {
	r.Get("/status", httpx.Wrap(h.status))
	r.Post("/connect", httpx.Wrap(h.connect))
	r.Delete("/connection", httpx.Wrap(h.disconnect))
	r.Post("/sync", httpx.Wrap(h.sync))
	r.Get("/unmatched", httpx.Wrap(h.listUnmatched))
	r.Post("/unmatched/{id}/resolve", httpx.Wrap(h.resolve))
	r.Post("/unmatched/{id}/dismiss", httpx.Wrap(h.dismiss))
}

// RegisterCallback mounts the public OAuth redirect target. It carries no
// Authorization header (it is a browser redirect from Google); identity comes
// from the single-use state created by an authenticated /gmail/connect call.
func (h *Handler) RegisterCallback(r chi.Router) {
	r.Get("/oauth/gmail/callback", h.callback)
}

func (h *Handler) require() error {
	if h.svc == nil {
		return apperr.New(http.StatusServiceUnavailable, "gmail_not_configured", "Gmail integration is not configured on this server")
	}
	return nil
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	if h.svc == nil {
		httpx.Data(w, http.StatusOK, AccountStatus{Enabled: false, RecentSyncs: []SyncRecord{}})
		return nil
	}
	st, err := h.svc.Status(r.Context(), userID)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, st)
	return nil
}

func (h *Handler) connect(w http.ResponseWriter, r *http.Request) error {
	if err := h.require(); err != nil {
		return err
	}
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	url, err := h.svc.StartConnect(r.Context(), userID)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, map[string]string{"url": url})
	return nil
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Error(w, "gmail integration is not configured", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	if q.Get("error") != "" { // user denied consent, or Google reported an error
		http.Redirect(w, r, h.svc.FrontendRedirect("denied"), http.StatusFound)
		return
	}
	status, err := h.svc.CompleteCallback(r.Context(), q.Get("code"), q.Get("state"))
	if err != nil {
		// The error never contains tokens or the authorization code.
		slog.WarnContext(r.Context(), "gmail oauth callback failed", "status", status, "error", err.Error())
	}
	http.Redirect(w, r, h.svc.FrontendRedirect(status), http.StatusFound)
}

func (h *Handler) disconnect(w http.ResponseWriter, r *http.Request) error {
	if err := h.require(); err != nil {
		return err
	}
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	if err := h.svc.Disconnect(r.Context(), userID); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) sync(w http.ResponseWriter, r *http.Request) error {
	if err := h.require(); err != nil {
		return err
	}
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	res, err := h.svc.Sync(r.Context(), userID)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, res)
	return nil
}

func (h *Handler) listUnmatched(w http.ResponseWriter, r *http.Request) error {
	if err := h.require(); err != nil {
		return err
	}
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	p, err := pagination.Parse(r)
	if err != nil {
		return err
	}
	items, total, err := h.svc.ListUnmatched(r.Context(), userID, httpx.QueryString(r, "status"), p)
	if err != nil {
		return err
	}
	httpx.List(w, items, pagination.NewMeta(p, total))
	return nil
}

type resolveInput struct {
	ContactID uuid.UUID `json:"contact_id"`
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) error {
	if err := h.require(); err != nil {
		return err
	}
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in resolveInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if err := h.svc.Resolve(r.Context(), userID, id, in.ContactID); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) error {
	if err := h.require(); err != nil {
		return err
	}
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Dismiss(r.Context(), userID, id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}
