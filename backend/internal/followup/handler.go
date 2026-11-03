package followup

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"referralautomation/internal/auth"
	"referralautomation/internal/platform/httpx"
	"referralautomation/internal/platform/pagination"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterJobContactScoped mounts routes on the /job-contacts router.
func (h *Handler) RegisterJobContactScoped(r chi.Router) {
	r.Post("/{id}/followup", httpx.Wrap(h.generate))
	r.Post("/{id}/followup/mark-sent", httpx.Wrap(h.markSent))
	r.Get("/{id}/messages", httpx.Wrap(h.listMessages))
}

// RegisterRoot mounts the top-level routes (followups, messages, templates, settings).
func (h *Handler) RegisterRoot(r chi.Router) {
	r.Get("/followups/due", httpx.Wrap(h.due))
	r.Put("/messages/{id}", httpx.Wrap(h.editMessage))
	r.Get("/templates", httpx.Wrap(h.listTemplates))
	r.Post("/templates", httpx.Wrap(h.createTemplate))
	r.Get("/templates/{id}", httpx.Wrap(h.getTemplate))
	r.Put("/templates/{id}", httpx.Wrap(h.updateTemplate))
	r.Delete("/templates/{id}", httpx.Wrap(h.deleteTemplate))
	r.Get("/settings", httpx.Wrap(h.getSettings))
	r.Put("/settings", httpx.Wrap(h.putSettings))
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in GenerateInput
	if err := httpx.DecodeOptional(w, r, &in); err != nil {
		return err
	}
	msg, created, err := h.svc.Generate(r.Context(), userID, id, in)
	if err != nil {
		return err
	}
	status := http.StatusOK // an existing draft was returned
	if created {
		status = http.StatusCreated
	}
	httpx.Data(w, status, msg)
	return nil
}

func (h *Handler) markSent(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	msg, status, err := h.svc.MarkSent(r.Context(), userID, id)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, map[string]any{"message": msg, "job_contact_status": status})
	return nil
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	msgs, err := h.svc.ListMessages(r.Context(), userID, id)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, msgs)
	return nil
}

func (h *Handler) due(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	p, err := pagination.Parse(r)
	if err != nil {
		return err
	}
	items, total, err := h.svc.ListDue(r.Context(), userID, p)
	if err != nil {
		return err
	}
	httpx.List(w, items, pagination.NewMeta(p, total))
	return nil
}

type editMessageInput struct {
	Body string `json:"body"`
}

func (h *Handler) editMessage(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in editMessageInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	msg, err := h.svc.EditDraft(r.Context(), userID, id, in.Body)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, msg)
	return nil
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	items, err := h.svc.ListTemplates(r.Context(), userID)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, items)
	return nil
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	var in TemplateInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	t, err := h.svc.CreateTemplate(r.Context(), userID, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusCreated, t)
	return nil
}

func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	t, err := h.svc.GetTemplate(r.Context(), userID, id)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, t)
	return nil
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in TemplateInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	t, err := h.svc.UpdateTemplate(r.Context(), userID, id, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, t)
	return nil
}

func (h *Handler) deleteTemplate(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteTemplate(r.Context(), userID, id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	s, err := h.svc.GetSettings(r.Context(), userID)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, s)
	return nil
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) error {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		return err
	}
	var in SettingsInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	s, err := h.svc.UpdateSettings(r.Context(), userID, in)
	if err != nil {
		return err
	}
	httpx.Data(w, http.StatusOK, s)
	return nil
}
