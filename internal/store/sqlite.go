package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Insert(ctx context.Context, trace Trace) error {
	const query = `
		INSERT INTO traces (
			id,
			trace_id,
			environment,
			server_name,
			method,
			params_hash,
			params_payload,
			response_hash,
			response_payload,
			latency_ms,
			is_error,
			error_message,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		trace.ID,
		trace.TraceID,
		trace.Environment,
		trace.ServerName,
		trace.Method,
		trace.ParamsHash,
		trace.ParamsPayload,
		trace.ResponseHash,
		trace.ResponsePayload,
		trace.LatencyMs,
		trace.IsError,
		trace.ErrorMessage,
		trace.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}

	return nil
}

func (s *SQLiteStore) Query(ctx context.Context, filter QueryFilter) ([]Trace, error) {
	var conditions []string
	var args []any

	if filter.TraceID != "" {
		conditions = append(conditions, "trace_id = ?")
		args = append(args, filter.TraceID)
	}
	if filter.Environment != "" {
		conditions = append(conditions, "environment = ?")
		args = append(args, filter.Environment)
	}
	if filter.ServerName != "" {
		conditions = append(conditions, "server_name = ?")
		args = append(args, filter.ServerName)
	}
	if filter.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, filter.Method)
	}
	if filter.IsError != nil {
		conditions = append(conditions, "is_error = ?")
		args = append(args, *filter.IsError)
	}
	if filter.CreatedAfter != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.CreatedAfter.UTC())
	}

	query := `
		SELECT
			id,
			trace_id,
			environment,
			server_name,
			method,
			params_hash,
			params_payload,
			response_hash,
			response_payload,
			latency_ms,
			is_error,
			error_message,
			created_at
		FROM traces
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	} else if filter.Offset > 0 {
		query += " LIMIT -1 OFFSET ?"
		args = append(args, filter.Offset)
	}

	return s.selectTraces(ctx, query, args...)
}

func (s *SQLiteStore) List(ctx context.Context, opts ListOptions) ([]Trace, error) {
	query := `
		SELECT
			id,
			trace_id,
			environment,
			server_name,
			method,
			params_hash,
			params_payload,
			response_hash,
			response_payload,
			latency_ms,
			is_error,
			error_message,
			created_at
		FROM traces
		ORDER BY created_at DESC
	`

	args := make([]any, 0, 2)
	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
		if opts.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, opts.Offset)
		}
	} else if opts.Offset > 0 {
		query += " LIMIT -1 OFFSET ?"
		args = append(args, opts.Offset)
	}

	return s.selectTraces(ctx, query, args...)
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM traces WHERE created_at < ?`, cutoff.UTC())
	if err != nil {
		return fmt.Errorf("delete old traces: %w", err)
	}
	return nil
}

func (s *SQLiteStore) TrimToCount(ctx context.Context, keep int) error {
	if keep <= 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM traces`)
		if err != nil {
			return fmt.Errorf("trim traces to zero: %w", err)
		}
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
		DELETE FROM traces
		WHERE id IN (
			SELECT id FROM traces
			ORDER BY created_at DESC
			LIMIT -1 OFFSET ?
		)
	`, keep)
	if err != nil {
		return fmt.Errorf("trim traces: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpsertAlertRule(ctx context.Context, rule AlertRule) (AlertRule, error) {
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now().UTC()
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_rules (
			id, name, rule_type, threshold, window_minutes, server_name, method, enabled, created_at, updated_at
			, environment
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			rule_type = excluded.rule_type,
			threshold = excluded.threshold,
			window_minutes = excluded.window_minutes,
			environment = excluded.environment,
			server_name = excluded.server_name,
			method = excluded.method,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`,
		rule.ID,
		rule.Name,
		rule.RuleType,
		rule.Threshold,
		rule.WindowMinutes,
		rule.ServerName,
		rule.Method,
		boolToInt(rule.Enabled),
		rule.CreatedAt.UTC(),
		rule.UpdatedAt.UTC(),
		rule.Environment,
	)
	if err != nil {
		return AlertRule{}, fmt.Errorf("upsert alert rule: %w", err)
	}

	return rule, nil
}

func (s *SQLiteStore) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, environment, name, rule_type, threshold, window_minutes, server_name, method, enabled, created_at, updated_at
		FROM alert_rules
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var rule AlertRule
		var enabled int
		if err := rows.Scan(
			&rule.ID,
			&rule.Environment,
			&rule.Name,
			&rule.RuleType,
			&rule.Threshold,
			&rule.WindowMinutes,
			&rule.ServerName,
			&rule.Method,
			&enabled,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rule.Enabled = enabled != 0
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}
	return rules, nil
}

func (s *SQLiteStore) DeleteAlertRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	return nil
}

func (s *SQLiteStore) InsertAlertEvent(ctx context.Context, event AlertEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_events (
			id, rule_id, environment, rule_name, status, previous_status, current_value, threshold, sample_count, notification, delivery_status, delivery_error, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		event.ID,
		event.RuleID,
		event.Environment,
		event.RuleName,
		event.Status,
		event.PreviousStatus,
		event.CurrentValue,
		event.Threshold,
		event.SampleCount,
		event.Notification,
		event.DeliveryStatus,
		event.DeliveryError,
		event.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert alert event: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListAlertEvents(ctx context.Context, environment string, limit int) ([]AlertEvent, error) {
	query := `
		SELECT id, rule_id, environment, rule_name, status, previous_status, current_value, threshold, sample_count, notification, delivery_status, delivery_error, created_at
		FROM alert_events
	`
	args := make([]any, 0, 2)
	if environment != "" {
		query += ` WHERE environment = ?`
		args = append(args, environment)
	}
	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alert events: %w", err)
	}
	defer rows.Close()

	var events []AlertEvent
	for rows.Next() {
		var event AlertEvent
		if err := rows.Scan(
			&event.ID,
			&event.RuleID,
			&event.Environment,
			&event.RuleName,
			&event.Status,
			&event.PreviousStatus,
			&event.CurrentValue,
			&event.Threshold,
			&event.SampleCount,
			&event.Notification,
			&event.DeliveryStatus,
			&event.DeliveryError,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert events: %w", err)
	}
	return events, nil
}

func (s *SQLiteStore) LatestAlertEvent(ctx context.Context, environment, ruleID string) (*AlertEvent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, rule_id, environment, rule_name, status, previous_status, current_value, threshold, sample_count, notification, delivery_status, delivery_error, created_at
		FROM alert_events
		WHERE rule_id = ? AND environment = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, ruleID, environment)

	var event AlertEvent
	if err := row.Scan(
		&event.ID,
		&event.RuleID,
		&event.Environment,
		&event.RuleName,
		&event.Status,
		&event.PreviousStatus,
		&event.CurrentValue,
		&event.Threshold,
		&event.SampleCount,
		&event.Notification,
		&event.DeliveryStatus,
		&event.DeliveryError,
		&event.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest alert event: %w", err)
	}
	return &event, nil
}

func (s *SQLiteStore) QueryLatencyStats(ctx context.Context, filter QueryFilter) ([]LatencyStat, error) {
	query, args := buildWindowedTraceFilter(filter)
	rows, err := s.db.QueryContext(ctx, `
		WITH filtered AS (
			SELECT environment, server_name, method, latency_ms,
				ROW_NUMBER() OVER (PARTITION BY environment, server_name, method ORDER BY latency_ms) AS rn,
				COUNT(*) OVER (PARTITION BY environment, server_name, method) AS total
			FROM traces
	`+query+`
		),
		grouped AS (
			SELECT
				environment,
				server_name,
				method,
				total AS count,
				MIN(CASE WHEN rn >= CAST(((total * 50) + 99) / 100 AS INTEGER) THEN latency_ms END) AS p50_ms,
				MIN(CASE WHEN rn >= CAST(((total * 95) + 99) / 100 AS INTEGER) THEN latency_ms END) AS p95_ms,
				MIN(CASE WHEN rn >= CAST(((total * 99) + 99) / 100 AS INTEGER) THEN latency_ms END) AS p99_ms
			FROM filtered
			GROUP BY environment, server_name, method, total
		)
		SELECT environment, server_name, method, count, COALESCE(p50_ms, 0), COALESCE(p95_ms, 0), COALESCE(p99_ms, 0)
		FROM grouped
		ORDER BY environment, server_name, method
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query latency stats: %w", err)
	}
	defer rows.Close()

	var stats []LatencyStat
	for rows.Next() {
		var stat LatencyStat
		if err := rows.Scan(&stat.Environment, &stat.ServerName, &stat.Method, &stat.Count, &stat.P50Ms, &stat.P95Ms, &stat.P99Ms); err != nil {
			return nil, fmt.Errorf("scan latency stat: %w", err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latency stats: %w", err)
	}
	return stats, nil
}

func (s *SQLiteStore) QueryErrorStats(ctx context.Context, filter QueryFilter) ([]ErrorStat, error) {
	query, args := buildWindowedTraceFilter(filter)
	rows, err := s.db.QueryContext(ctx, `
		WITH filtered AS (
			SELECT
				environment,
				method,
				is_error,
				error_message,
				created_at,
				ROW_NUMBER() OVER (
					PARTITION BY environment, method
					ORDER BY CASE WHEN is_error = 1 THEN created_at END DESC
				) AS err_rank
			FROM traces
	`+query+`
		)
		SELECT
			environment,
			method,
			COUNT(*) AS count,
			SUM(CASE WHEN is_error = 1 THEN 1 ELSE 0 END) AS error_count,
			CASE WHEN COUNT(*) = 0 THEN 0 ELSE (SUM(CASE WHEN is_error = 1 THEN 1 ELSE 0 END) * 100.0 / COUNT(*)) END AS error_rate_pct,
			MAX(CASE WHEN is_error = 1 AND err_rank = 1 THEN error_message ELSE '' END) AS recent_error_message,
			MAX(CASE WHEN is_error = 1 AND err_rank = 1 THEN created_at END) AS recent_error_at
		FROM filtered
		WHERE method <> ''
		GROUP BY environment, method
		ORDER BY error_rate_pct DESC, method
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query error stats: %w", err)
	}
	defer rows.Close()

	var stats []ErrorStat
	for rows.Next() {
		var stat ErrorStat
		var recentAt sql.NullString
		if err := rows.Scan(&stat.Environment, &stat.Method, &stat.Count, &stat.ErrorCount, &stat.ErrorRatePct, &stat.RecentErrorMessage, &recentAt); err != nil {
			return nil, fmt.Errorf("scan error stat: %w", err)
		}
		if recentAt.Valid {
			parsed, err := parseSQLiteTime(recentAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse recent error time: %w", err)
			}
			stat.RecentErrorAt = &parsed
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate error stats: %w", err)
	}
	return stats, nil
}

func (s *SQLiteStore) selectTraces(ctx context.Context, query string, args ...any) ([]Trace, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query traces: %w", err)
	}
	defer rows.Close()

	var traces []Trace
	for rows.Next() {
		var trace Trace
		if err := rows.Scan(
			&trace.ID,
			&trace.TraceID,
			&trace.Environment,
			&trace.ServerName,
			&trace.Method,
			&trace.ParamsHash,
			&trace.ParamsPayload,
			&trace.ResponseHash,
			&trace.ResponsePayload,
			&trace.LatencyMs,
			&trace.IsError,
			&trace.ErrorMessage,
			&trace.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan trace: %w", err)
		}
		traces = append(traces, trace)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate traces: %w", err)
	}

	return traces, nil
}

func runMigrations(db *sql.DB) error {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	databaseDriver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("create sqlite migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", databaseDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func buildWindowedTraceFilter(filter QueryFilter) (string, []any) {
	var conditions []string
	var args []any
	if filter.Environment != "" {
		conditions = append(conditions, "environment = ?")
		args = append(args, filter.Environment)
	}
	if filter.ServerName != "" {
		conditions = append(conditions, "server_name = ?")
		args = append(args, filter.ServerName)
	}
	if filter.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, filter.Method)
	}
	if filter.CreatedAfter != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.CreatedAfter.UTC())
	}
	if filter.IsError != nil {
		conditions = append(conditions, "is_error = ?")
		args = append(args, *filter.IsError)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func parseSQLiteTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", value)
}
