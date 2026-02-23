package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations executes all .sql files in the given directory in sorted order.
// Each file is executed inside a single transaction. Errors from individual
// statements (e.g. "column already exists") are logged but do not abort the run.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		path := filepath.Join(migrationsDir, name)
		sql, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		_, execErr := pool.Exec(ctx, string(sql))
		if execErr != nil {
			// Non-fatal: log and continue (handles "column already exists" etc.)
			fmt.Printf("  migration %s: %v (non-fatal, continuing)\n", name, execErr)
			continue
		}
		fmt.Printf("  migration %s: applied\n", name)
	}

	return nil
}
