package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ApplySeed(ctx context.Context, pool *pgxpool.Pool, seedDir string) error {
	if _, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS seed_history (
filename TEXT PRIMARY KEY,
applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`); err != nil {
		return err
	}

	entries, err := os.ReadDir(seedDir)
	if err != nil {
		return err
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, filename := range files {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seed_history WHERE filename = $1)`, filename).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		script, err := os.ReadFile(filepath.Join(seedDir, filename))
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(script)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("seed %s failed: %w", filename, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO seed_history(filename) VALUES ($1)`, filename); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
