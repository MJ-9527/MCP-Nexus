package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"MCP-Nexus/config"
)

func main() {
	dir := flag.String("dir", "db/migrations", "migration directory")
	seed := flag.String("seed", "", "optional seed SQL file")
	flag.Parse()
	config.Load()
	ctx := context.Background()
	pool, err := config.NewPostgresPool(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		log.Fatal(err)
	}
	entries, err := os.ReadDir(*dir)
	if err != nil {
		log.Fatal(err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		var applied bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&applied); err != nil {
			log.Fatal(err)
		}
		if applied {
			fmt.Printf("skip %s\n", name)
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(*dir, name))
		if readErr != nil {
			log.Fatal(readErr)
		}
		tx, beginErr := pool.Begin(ctx)
		if beginErr != nil {
			log.Fatal(beginErr)
		}
		started := time.Now()
		if _, err = tx.Exec(ctx, string(data)); err != nil {
			tx.Rollback(ctx)
			log.Fatalf("migration %s failed: %v", name, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, name); err != nil {
			tx.Rollback(ctx)
			log.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("apply %s (%s)\n", name, time.Since(started).Round(time.Millisecond))
	}
	if *seed != "" {
		data, readErr := os.ReadFile(*seed)
		if readErr != nil {
			log.Fatal(readErr)
		}
		if _, err = pool.Exec(ctx, string(data)); err != nil {
			log.Fatalf("seed failed: %v", err)
		}
		fmt.Printf("seed %s\n", *seed)
	}
}
