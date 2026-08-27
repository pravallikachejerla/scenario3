// Command migrate applies forward SQL migrations from the migrations
// directory against DATABASE_URL.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// The "pgx" driver is registered by driver_postgres.go, built only
// with `-tags postgres` (see that file). Without the tag, sql.Open
// below fails at runtime with "unknown driver" rather than at build
// time, so this command still compiles standalone with the Go
// standard library only.

func main() {
	dir := flag.String("dir", "migrations", "migrations directory")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://orders:orders@localhost:5433/orders?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()

	entries, err := os.ReadDir(*dir)
	if err != nil {
		log.Fatalf("read dir: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" && !isDown(e.Name()) {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	for _, f := range files {
		var exists bool
		_ = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)`, f).Scan(&exists)
		if exists {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*dir, f))
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}
		if _, err := db.ExecContext(ctx, string(b)); err != nil {
			log.Fatalf("apply %s: %v", f, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, f); err != nil {
			log.Fatalf("record %s: %v", f, err)
		}
		log.Printf("applied %s", f)
	}
	log.Println("migrations complete")
}

func isDown(name string) bool {
	return len(name) > 8 && name[len(name)-8:] == "down.sql"
}
