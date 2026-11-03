// Package validate contains tiny, dependency-free input validation helpers.
package validate

import (
	"net/mail"
	"net/url"
	"strings"
	"unicode/utf8"

	"referralautomation/internal/platform/apperr"
)

// Errors collects field -> message pairs.
type Errors map[string]string

func (e Errors) Add(field, msg string) {
	if _, exists := e[field]; !exists {
		e[field] = msg
	}
}

// Err returns nil when no errors were collected.
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return apperr.Validation(map[string]string(e))
}

func Trim(s string) string { return strings.TrimSpace(s) }

// TrimPtr trims and converts blank strings to nil.
func TrimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func Required(e Errors, field, v string) {
	if strings.TrimSpace(v) == "" {
		e.Add(field, "is required")
	}
}

func MaxLen(e Errors, field, v string, n int) {
	if utf8.RuneCountInString(v) > n {
		e.Add(field, "must be at most "+itoa(n)+" characters")
	}
}

func MaxLenPtr(e Errors, field string, v *string, n int) {
	if v != nil {
		MaxLen(e, field, *v, n)
	}
}

func OneOf(e Errors, field, v string, allowed ...string) {
	for _, a := range allowed {
		if v == a {
			return
		}
	}
	e.Add(field, "must be one of: "+strings.Join(allowed, ", "))
}

// OptionalURL accepts only absolute http(s) URLs.
func OptionalURL(e Errors, field string, v *string) {
	if v == nil {
		return
	}
	if len(*v) > 2048 {
		e.Add(field, "must be at most 2048 characters")
		return
	}
	u, err := url.Parse(*v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		e.Add(field, "must be a valid http(s) URL")
	}
}

// OptionalEmail accepts a bare address (no display name).
func OptionalEmail(e Errors, field string, v *string) {
	if v == nil {
		return
	}
	a, err := mail.ParseAddress(*v)
	if err != nil || a.Address != *v || len(*v) > 254 {
		e.Add(field, "must be a valid email address")
	}
}

// EscapeLike escapes %, _ and \ so user input is matched literally by ILIKE.
func EscapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
