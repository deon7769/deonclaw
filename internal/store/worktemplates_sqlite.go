package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/worktemplates"
)

func (s *SQLiteStore) bootstrapWorkTemplates(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS work_templates (
			id TEXT PRIMARY KEY,
			config_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap work templates: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SyncWorkTemplates(ctx context.Context, templates []worktemplates.Template) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := formatTime(time.Now().UTC())
	for _, tmpl := range templates {
		data, err := json.Marshal(tmpl)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO work_templates (id, config_json, created_at, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET config_json = excluded.config_json, updated_at = excluded.updated_at`,
			tmpl.ID, string(data), now, now,
		)
		if err != nil {
			return fmt.Errorf("sync work template %q: %w", tmpl.ID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListWorkTemplates(ctx context.Context) ([]worktemplates.Template, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT config_json FROM work_templates ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]worktemplates.Template, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var tmpl worktemplates.Template
		if err := json.Unmarshal([]byte(raw), &tmpl); err != nil {
			return nil, err
		}
		out = append(out, tmpl)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) WorkTemplate(ctx context.Context, id string) (worktemplates.Template, error) {
	row := s.db.QueryRowContext(ctx, `SELECT config_json FROM work_templates WHERE id = ?`, id)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return worktemplates.Template{}, fmt.Errorf("work template %q: %w", id, ErrNotFound)
		}
		return worktemplates.Template{}, err
	}
	var tmpl worktemplates.Template
	if err := json.Unmarshal([]byte(raw), &tmpl); err != nil {
		return worktemplates.Template{}, err
	}
	return tmpl, nil
}
