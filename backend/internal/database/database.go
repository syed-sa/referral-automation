// Package database provides the pgx pool, migration runner and error helpers.
package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"referralautomation/migrations"
)

// NewPool creates a pgx pool. QueryExecModeCacheDescribe avoids named
// prepared statements, so it works behind Supabase's transaction pooler
// (PgBouncer in transaction mode) as well as direct connections.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe
	cfg.MaxConns = 10
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// ---- migrations ----

func newMigrator(url string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toPgx5URL(url))
	if err != nil {
		return nil, fmt.Errorf("init migrator: %w", err)
	}
	return m, nil
}

// toPgx5URL rewrites postgres:// to the scheme golang-migrate's pgx v5 driver registers.
func toPgx5URL(u string) string {
	for _, p := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(u, p) {
			return "pgx5://" + strings.TrimPrefix(u, p)
		}
	}
	return u
}

func MigrateUp(url string) error {
	m, err := newMigrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateDown rolls back the given number of migrations.
func MigrateDown(url string, steps int) error {
	m, err := newMigrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func MigrateVersion(url string) (version uint, dirty bool, err error) {
	m, err := newMigrator(url)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, d, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return v, d, err
}

// ---- error helpers ----

const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
)

func pgErr(err error) *pgconn.PgError {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe
	}
	return nil
}

func IsUniqueViolation(err error) bool {
	pe := pgErr(err)
	return pe != nil && pe.Code == codeUniqueViolation
}

func IsForeignKeyViolation(err error) bool {
	pe := pgErr(err)
	return pe != nil && pe.Code == codeForeignKeyViolation
}

// ConstraintName returns the violated constraint, or "" when err is not a PgError.
func ConstraintName(err error) string {
	if pe := pgErr(err); pe != nil {
		return pe.ConstraintName
	}
	return ""
}
