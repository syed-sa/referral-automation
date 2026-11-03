// Package apperr defines the application error type that handlers map to
// HTTP responses. Services and repositories return *Error for expected
// failures; anything else is treated as an internal error.
package apperr

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
	Err     error // underlying cause, logged but never sent to the client
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "bad_request", message)
}

func Validation(details map[string]string) *Error {
	e := New(http.StatusUnprocessableEntity, "validation_failed", "request validation failed")
	e.Details = details
	return e
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "unauthorized", message)
}

// NotFound is also used for resources owned by another user, so existence of
// other tenants' data is never revealed.
func NotFound(resource string) *Error {
	return New(http.StatusNotFound, "not_found", resource+" not found")
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "conflict", message)
}

func Internal(err error) *Error {
	e := New(http.StatusInternalServerError, "internal_error", "internal server error")
	e.Err = err
	return e
}

// As reports whether err contains an *Error.
func As(err error) (*Error, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
