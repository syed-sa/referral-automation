// Package httpx holds HTTP helpers: JSON responses, strict request decoding,
// error mapping and path/query parsing.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/pagination"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Handler is an http handler that returns an error. Wrap converts it to a
// standard http.HandlerFunc and centralizes error rendering.
type Handler func(w http.ResponseWriter, r *http.Request) error

func Wrap(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}

type errorPayload struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ae, ok := apperr.As(err)
	if !ok {
		ae = apperr.Internal(err)
	}
	if ae.Status >= 500 {
		slog.ErrorContext(r.Context(), "request failed",
			"error", err.Error(),
			"request_id", chimw.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
	JSON(w, ae.Status, map[string]any{"error": errorPayload{
		Code:      ae.Code,
		Message:   ae.Message,
		Details:   ae.Details,
		RequestID: chimw.GetReqID(r.Context()),
	}})
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

// Data writes {"data": v}.
func Data(w http.ResponseWriter, status int, v any) {
	JSON(w, status, map[string]any{"data": v})
}

// List writes {"data": [...], "pagination": {...}}.
func List[T any](w http.ResponseWriter, items []T, meta pagination.Meta) {
	if items == nil {
		items = []T{}
	}
	JSON(w, http.StatusOK, struct {
		Data       []T             `json:"data"`
		Pagination pagination.Meta `json:"pagination"`
	}{Data: items, Pagination: meta})
}

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Decode reads exactly one JSON object. Unknown fields are rejected, which
// also means a client-supplied "user_id" is refused outright.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	return decode(w, r, dst, false)
}

// DecodeOptional is like Decode but accepts an empty body (dst is left untouched).
func DecodeOptional(w http.ResponseWriter, r *http.Request, dst any) error {
	return decode(w, r, dst, true)
}

func decode(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syn *json.SyntaxError
		var typ *json.UnmarshalTypeError
		var tooBig *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			if allowEmpty {
				return nil
			}
			return apperr.BadRequest("request body must not be empty")
		case errors.As(err, &tooBig):
			return apperr.New(http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large")
		case errors.As(err, &syn), errors.Is(err, io.ErrUnexpectedEOF):
			return apperr.BadRequest("malformed JSON")
		case errors.As(err, &typ):
			return apperr.BadRequest(fmt.Sprintf("invalid value for field %q", typ.Field))
		default:
			return apperr.BadRequest(err.Error())
		}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.BadRequest("request body must contain a single JSON object")
	}
	return nil
}

func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperr.BadRequest("invalid " + name)
	}
	return id, nil
}

// QueryUUID returns nil when the parameter is absent.
func QueryUUID(r *http.Request, key string) (*uuid.UUID, error) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil, apperr.BadRequest("invalid " + key)
	}
	return &id, nil
}

// QueryString returns nil when the parameter is absent or blank.
func QueryString(r *http.Request, key string) *string {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return nil
	}
	return &v
}
