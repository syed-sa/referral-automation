// Command migrate applies embedded SQL migrations.
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down      # rolls back ONE migration
//	go run ./cmd/migrate version
//
// Uses MIGRATION_DATABASE_URL (a direct, non-pooled connection) when set,
// otherwise DATABASE_URL.
package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"referralautomation/internal/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	url := os.Getenv("MIGRATION_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return fmt.Errorf("MIGRATION_DATABASE_URL or DATABASE_URL must be set")
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <up|down|version>")
	}

	switch os.Args[1] {
	case "up":
		if err := database.MigrateUp(url); err != nil {
			return err
		}
		fmt.Println("migrations applied")
	case "down":
		if err := database.MigrateDown(url, 1); err != nil {
			return err
		}
		fmt.Println("rolled back one migration")
	case "version":
		v, dirty, err := database.MigrateVersion(url)
		if err != nil {
			return err
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
	default:
		return fmt.Errorf("unknown command %q (use up, down or version)", os.Args[1])
	}
	return nil
}
