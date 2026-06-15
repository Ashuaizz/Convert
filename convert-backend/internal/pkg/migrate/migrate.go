package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func Up(ctx context.Context, dsn string, dir string) error {
	if dsn == "" {
		return fmt.Errorf("database dsn is required")
	}
	if dir == "" {
		dir = "migrations"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	return UpWithDB(ctx, db, dir)
}

func UpWithDB(ctx context.Context, db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("execute migration %s: %w", file, err)
		}
	}
	return nil
}
