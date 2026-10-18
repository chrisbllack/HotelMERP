package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"infineon/internal/config"
	"infineon/internal/database"
	"infineon/internal/database/sqlite"
	"infineon/internal/server"
)

// HotelMERP on-premise server entrypoint.
//
// Storage backends:
//   - sqlite (default): embedded single-file database — zero external
//     dependencies; schema + demo seed run automatically on first boot
//   - postgres: set DATABASE_URL=postgres://... — migrations/ and seeds/
//     are applied on boot
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[main] config: %v", err)
	}

	ctx := context.Background()
	pool, err := openStorage(cfg, ctx)
	if err != nil {
		log.Fatalf("[main] storage: %v", err)
	}

	router := server.New(cfg, pool, nil)

	addr := ":" + cfg.Port
	log.Printf("HotelMERP listening on http://localhost%s (mode=%s, payment=%s)", addr, cfg.Mode, cfg.PaymentProvider)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

// openStorage selects the storage backend. SQLite is the default so the
// system runs on-premise with no external database service.
func openStorage(cfg *config.Config, ctx context.Context) (database.Pool, error) {
	// Postgres only when DATABASE_URL explicitly points at postgres.
	if strings.HasPrefix(os.Getenv("DATABASE_URL"), "postgres") {
		pg, err := database.Connect(cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		if err := database.Migrate(pg); err != nil {
			return nil, err
		}
		if err := database.Seed(pg); err != nil {
			return nil, err
		}
		return database.NewPGXPool(pg), nil
	}

	dbPath := os.Getenv("INFINEON_DB_PATH")
	if dbPath == "" {
		dbPath = "data/hotelmerp.db"
	}

	fresh := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fresh = true
	}

	db, err := sqlite.New(dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.RunMigrations(ctx); err != nil {
		return nil, err
	}
	if fresh {
		if err := db.SeedDemoData(ctx); err != nil {
			return nil, err
		}
		log.Printf("[main] seeded demo data into %s", dbPath)
	}
	log.Printf("[main] SQLite storage ready at %s", dbPath)
	return database.NewSQLitePool(db.DB), nil
}
