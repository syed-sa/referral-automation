// Package auth verifies Supabase JWTs and carries the authenticated principal
// through the request context.
package auth

import (
	"context"

	"github.com/google/uuid"

	"referralautomation/internal/platform/apperr"
)

// Principal is the authenticated caller. It is only ever built from a
// validated JWT; user ids from request bodies or query strings are never used.
type Principal struct {
	UserID uuid.UUID
	Email  string
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// UserID returns the authenticated user's id, or a 401 error.
func UserID(ctx context.Context) (uuid.UUID, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok || p.UserID == uuid.Nil {
		return uuid.Nil, apperr.Unauthorized("authentication required")
	}
	return p.UserID, nil
}
