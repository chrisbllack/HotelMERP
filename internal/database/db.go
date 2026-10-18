package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(databaseURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	log.Println("Connected to PostgreSQL")
	return pool, nil
}

func Migrate(db *pgxpool.Pool) error {
	ctx := context.Background()

	// Create schema_migrations table
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	// Get applied migrations
	applied := make(map[string]bool)
	rows, err := db.Query(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var f string
		rows.Scan(&f)
		applied[f] = true
	}
	rows.Close()

	// Find migration files
	migrations := []string{
		"0001_core.up.sql",
		"0002_identity.up.sql",
		"0003_organization.up.sql",
		"0004_configuration.up.sql",
		"0005_audit.up.sql",
		"0006_hospitality.up.sql",
		"0007_pos.up.sql",
		"0008_inventory.up.sql",
		"0009_finance.up.sql",
		"0010_hr.up.sql",
		"0011_reports.up.sql",
		"0012_channel_payment_hardware.up.sql",
		"0013_crm_events.up.sql",
		"0014_advanced_modules.up.sql",
		"0015_identity_bridge.up.sql",
		"0016_contract_bus.up.sql",
	}

	sort.Strings(migrations)
	appliedCount := 0

	for _, file := range migrations {
		if applied[file] {
			continue
		}

		content, err := os.ReadFile(filepath.Join("migrations", file))
		if err != nil {
			log.Printf("  Skipping %s: %v", file, err)
			continue
		}

		log.Printf("  Applying %s...", file)
		statements := splitSQL(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if _, err := db.Exec(ctx, stmt); err != nil {
				// Log but continue — IF NOT EXISTS handles duplicates
				if !strings.Contains(err.Error(), "already exists") {
					log.Printf("  Warning: %v", err)
				}
			}
		}

		_, _ = db.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`, file)
		appliedCount++
	}

	if appliedCount > 0 {
		log.Printf("Applied %d migration(s)", appliedCount)
	}
	return nil
}

func Seed(db *pgxpool.Pool) error {
	ctx := context.Background()
	seedFiles := []string{
		"001_currencies.sql",
		"002_countries.sql",
		"003_units.sql",
		"004_account_classes.sql",
		"005_demo_hotel.sql",
		"006_pos_data.sql",
		"007_demo_users.sql",
		"008_comprehensive_demo.sql",
		"009_complete_hotel_data.sql",
	}

	for _, file := range seedFiles {
		path := filepath.Join("seeds", file)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		log.Printf("  Seeding %s...", file)
		statements := splitSQL(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if _, err := db.Exec(ctx, stmt); err != nil {
				if !strings.Contains(err.Error(), "duplicate key") &&
					!strings.Contains(err.Error(), "already exists") {
					log.Printf("  Seed warning: %v", err)
				}
			}
		}
	}
	return nil
}

func Backup(db *pgxpool.Pool, path string) error {
	log.Printf("Backup to %s...", path)
	// In production: use pg_dump via exec
	return nil
}

func splitSQL(content string) []string {
	var statements []string
	var current strings.Builder

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		statements = append(statements, current.String())
	}
	return statements
}
