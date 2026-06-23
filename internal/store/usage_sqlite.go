package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/usage"
)

func (s *SQLiteStore) bootstrapModelPrices(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS model_prices (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			currency TEXT NOT NULL DEFAULT 'USD',
			input_microusd_per_million INTEGER NOT NULL DEFAULT 0,
			output_microusd_per_million INTEGER NOT NULL DEFAULT 0,
			cached_input_microusd_per_million INTEGER NOT NULL DEFAULT 0,
			effective_at TEXT NOT NULL,
			config_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_model_prices_provider_model ON model_prices(provider, model)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap model prices: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) bootstrapUsageEvents(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS usage_events (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			work_item_id TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			session_id TEXT NOT NULL DEFAULT '',
			worker TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			model_profile TEXT NOT NULL DEFAULT '',
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			tool_call_count INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			estimated_cost_microusd INTEGER NOT NULL DEFAULT 0,
			actual_cost_microusd INTEGER NOT NULL DEFAULT 0,
			source TEXT NOT NULL,
			confidence TEXT NOT NULL,
			tokens_available INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			config_json TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_run ON usage_events(run_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_agent ON usage_events(agent_id, created_at)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap usage events: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SyncModelPrices(ctx context.Context, prices []usage.ModelPrice) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := formatTime(time.Now().UTC())
	for _, price := range prices {
		configJSON, err := json.Marshal(map[string]string{})
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO model_prices (
			id, provider, model, currency, input_microusd_per_million, output_microusd_per_million,
			cached_input_microusd_per_million, effective_at, config_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			currency = excluded.currency,
			input_microusd_per_million = excluded.input_microusd_per_million,
			output_microusd_per_million = excluded.output_microusd_per_million,
			cached_input_microusd_per_million = excluded.cached_input_microusd_per_million,
			effective_at = excluded.effective_at,
			config_json = excluded.config_json,
			updated_at = excluded.updated_at`,
			price.ID, price.Provider, price.Model, nullIfEmpty(price.Currency),
			price.InputMicroUSDPerMillion, price.OutputMicroUSDPerMillion, price.CachedInputMicroUSDPerMillion,
			price.EffectiveAt, string(configJSON), now, now,
		)
		if err != nil {
			return fmt.Errorf("sync model price %q: %w", price.ID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListModelPrices(ctx context.Context) ([]usage.ModelPrice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, model, currency, input_microusd_per_million,
		output_microusd_per_million, cached_input_microusd_per_million, effective_at
		FROM model_prices ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]usage.ModelPrice, 0)
	for rows.Next() {
		var price usage.ModelPrice
		if err := rows.Scan(&price.ID, &price.Provider, &price.Model, &price.Currency,
			&price.InputMicroUSDPerMillion, &price.OutputMicroUSDPerMillion, &price.CachedInputMicroUSDPerMillion,
			&price.EffectiveAt); err != nil {
			return nil, err
		}
		out = append(out, price)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ModelPrice(ctx context.Context, id string) (usage.ModelPrice, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, provider, model, currency, input_microusd_per_million,
		output_microusd_per_million, cached_input_microusd_per_million, effective_at
		FROM model_prices WHERE id = ?`, id)
	var price usage.ModelPrice
	if err := row.Scan(&price.ID, &price.Provider, &price.Model, &price.Currency,
		&price.InputMicroUSDPerMillion, &price.OutputMicroUSDPerMillion, &price.CachedInputMicroUSDPerMillion,
		&price.EffectiveAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return usage.ModelPrice{}, fmt.Errorf("model price %q: %w", id, ErrNotFound)
		}
		return usage.ModelPrice{}, err
	}
	return price, nil
}

func (s *SQLiteStore) SaveUsageEvent(ctx context.Context, event usage.Event) error {
	if err := usage.ValidateEvent(event); err != nil {
		return err
	}
	configJSON, _ := json.Marshal(map[string]any{})
	_, err := s.db.ExecContext(ctx, `INSERT INTO usage_events (
		id, run_id, work_item_id, agent_id, session_id, worker, provider, model, model_profile,
		input_tokens, output_tokens, cached_input_tokens, tool_call_count, duration_ms,
		estimated_cost_microusd, actual_cost_microusd, source, confidence, tokens_available, created_at, config_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		actual_cost_microusd = excluded.actual_cost_microusd,
		estimated_cost_microusd = excluded.estimated_cost_microusd,
		config_json = excluded.config_json`,
		event.ID, event.RunID, nullIfEmpty(event.WorkItemID), nullIfEmpty(event.AgentID), nullIfEmpty(event.SessionID),
		event.Worker, nullIfEmpty(event.Provider), nullIfEmpty(event.Model), nullIfEmpty(event.ModelProfile),
		event.InputTokens, event.OutputTokens, event.CachedInputTokens, event.ToolCallCount, event.DurationMS,
		event.EstimatedCostMicroUSD, event.ActualCostMicroUSD, event.Source, event.Confidence, boolToInt(event.TokensAvailable),
		event.CreatedAt, string(configJSON),
	)
	if err != nil {
		return fmt.Errorf("save usage event %q: %w", event.ID, err)
	}
	return nil
}

func (s *SQLiteStore) UsageEvent(ctx context.Context, id string) (usage.Event, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, run_id, work_item_id, agent_id, session_id, worker, provider, model, model_profile,
		input_tokens, output_tokens, cached_input_tokens, tool_call_count, duration_ms,
		estimated_cost_microusd, actual_cost_microusd, source, confidence, tokens_available, created_at
		FROM usage_events WHERE id = ?`, id)
	event, err := scanUsageEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return usage.Event{}, fmt.Errorf("usage event %q: %w", id, ErrNotFound)
	}
	return event, err
}

func (s *SQLiteStore) ListUsageEvents(ctx context.Context, since *time.Time) ([]usage.Event, error) {
	query := `SELECT id, run_id, work_item_id, agent_id, session_id, worker, provider, model, model_profile,
		input_tokens, output_tokens, cached_input_tokens, tool_call_count, duration_ms,
		estimated_cost_microusd, actual_cost_microusd, source, confidence, tokens_available, created_at
		FROM usage_events`
	args := []any{}
	if since != nil && !since.IsZero() {
		query += ` WHERE created_at >= ?`
		args = append(args, formatTime(since.UTC()))
	}
	query += ` ORDER BY created_at, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]usage.Event, 0)
	for rows.Next() {
		event, err := scanUsageEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func scanUsageEvent(scanner interface{ Scan(...any) error }) (usage.Event, error) {
	var event usage.Event
	var tokensAvailable int
	if err := scanner.Scan(
		&event.ID, &event.RunID, &event.WorkItemID, &event.AgentID, &event.SessionID,
		&event.Worker, &event.Provider, &event.Model, &event.ModelProfile,
		&event.InputTokens, &event.OutputTokens, &event.CachedInputTokens, &event.ToolCallCount, &event.DurationMS,
		&event.EstimatedCostMicroUSD, &event.ActualCostMicroUSD, &event.Source, &event.Confidence, &tokensAvailable, &event.CreatedAt,
	); err != nil {
		return usage.Event{}, err
	}
	event.TokensAvailable = tokensAvailable != 0
	return event, nil
}
