// Command devtoken mints a short-lived HS256 JWT for LOCAL development so the
// API can be exercised with curl without a Supabase project.
//
//	go run ./cmd/devtoken -sub 11111111-1111-1111-1111-111111111111 -email me@example.com
//
// It requires SUPABASE_JWT_SECRET (the same value the API is started with) and
// refuses to run when APP_ENV=production.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		return fmt.Errorf("devtoken is disabled when APP_ENV=production")
	}

	sub := flag.String("sub", "11111111-1111-1111-1111-111111111111", "user id (UUID) to put in the sub claim")
	email := flag.String("email", "dev@example.com", "email claim")
	ttl := flag.Duration("ttl", time.Hour, "token lifetime")
	flag.Parse()

	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		return fmt.Errorf("SUPABASE_JWT_SECRET must be set")
	}
	if _, err := uuid.Parse(*sub); err != nil {
		return fmt.Errorf("-sub must be a UUID: %w", err)
	}
	issuer := os.Getenv("SUPABASE_JWT_ISSUER")
	if issuer == "" {
		base := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
		if base == "" {
			return fmt.Errorf("SUPABASE_URL (or SUPABASE_JWT_ISSUER) must be set")
		}
		issuer = base + "/auth/v1"
	}
	aud := os.Getenv("SUPABASE_JWT_AUDIENCE")
	if aud == "" {
		aud = "authenticated"
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   issuer,
		"aud":   aud,
		"sub":   *sub,
		"role":  "authenticated",
		"email": *email,
		"iat":   now.Unix(),
		"exp":   now.Add(*ttl).Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}
