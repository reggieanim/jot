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

func ResolveMigrationsDir(preferred string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(preferred) != "" {
		candidates = append(candidates, strings.TrimSpace(preferred))
	}

	candidates = append(candidates,
		"/migrations",
		"migrations",
		"./migrations",
		"../migrations",
		"../../migrations",
	)

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}

		entries, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("no migrations directory found (checked: %s)", strings.Join(candidates, ", "))
}

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
