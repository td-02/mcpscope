package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreInsertAndReadBackTrace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "traces.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite returned error: %v", err)
	}
	defer store.Close()

	createdAt := time.Date(2026, 3, 19, 9, 30, 0, 0, time.UTC)
	trace := Trace{
		ID:              "550e8400-e29b-41d4-a716-446655440000",
		TraceID:         "8f14e45f-ea4c-4d57-8b55-6e5d7f1db7b1",
		ServerName:      "demo-server",
		Method:          "tools/call",
		ParamsHash:      "params-hash",
		ParamsPayload:   `{"name":"ping"}`,
		ResponseHash:    "response-hash",
		ResponsePayload: `{"ok":true}`,
		LatencyMs:       25,
		IsError:         true,
		ErrorMessage:    "boom",
		CreatedAt:       createdAt,
	}

	if err := store.Insert(ctx, trace); err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	listed, err := store.List(ctx, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed trace, got %d", len(listed))
	}

	got := listed[0]
	if got.ID != trace.ID {
		t.Fatalf("id = %q, want %q", got.ID, trace.ID)
	}
	if got.TraceID != trace.TraceID {
		t.Fatalf("trace_id = %q, want %q", got.TraceID, trace.TraceID)
	}
	if got.ServerName != trace.ServerName {
		t.Fatalf("server_name = %q, want %q", got.ServerName, trace.ServerName)
	}
	if got.Method != trace.Method {
		t.Fatalf("method = %q, want %q", got.Method, trace.Method)
	}
	if got.ParamsHash != trace.ParamsHash {
		t.Fatalf("params_hash = %q, want %q", got.ParamsHash, trace.ParamsHash)
	}
	if got.ResponseHash != trace.ResponseHash {
		t.Fatalf("response_hash = %q, want %q", got.ResponseHash, trace.ResponseHash)
	}
	if got.ParamsPayload != trace.ParamsPayload {
		t.Fatalf("params_payload = %q, want %q", got.ParamsPayload, trace.ParamsPayload)
	}
	if got.ResponsePayload != trace.ResponsePayload {
		t.Fatalf("response_payload = %q, want %q", got.ResponsePayload, trace.ResponsePayload)
	}
	if got.LatencyMs != trace.LatencyMs {
		t.Fatalf("latency_ms = %d, want %d", got.LatencyMs, trace.LatencyMs)
	}
	if got.IsError != trace.IsError {
		t.Fatalf("is_error = %v, want %v", got.IsError, trace.IsError)
	}
	if got.ErrorMessage != trace.ErrorMessage {
		t.Fatalf("error_message = %q, want %q", got.ErrorMessage, trace.ErrorMessage)
	}

	filtered, err := store.Query(ctx, QueryFilter{TraceID: trace.TraceID, Limit: 1})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 queried trace, got %d", len(filtered))
	}
	if filtered[0].CreatedAt.UTC().Format(time.RFC3339Nano) != createdAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("created_at = %s, want %s", filtered[0].CreatedAt.UTC().Format(time.RFC3339Nano), createdAt.UTC().Format(time.RFC3339Nano))
	}
}

func TestSQLiteStoreRetentionAndAlertRules(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "traces.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite returned error: %v", err)
	}
	defer store.Close()

	for i := 0; i < 3; i++ {
		ts := time.Date(2026, 3, 20, 9, 0, i, 0, time.UTC)
		if err := store.Insert(ctx, Trace{
			ID:              fmt.Sprintf("trace-%d", i),
			TraceID:         fmt.Sprintf("correlated-%d", i),
			ServerName:      "demo-server",
			Method:          "tools/call",
			ParamsHash:      "params",
			ParamsPayload:   `{}`,
			ResponseHash:    "resp",
			ResponsePayload: `{}`,
			LatencyMs:       int64(10 + i),
			CreatedAt:       ts,
		}); err != nil {
			t.Fatalf("Insert returned error: %v", err)
		}
	}

	if err := store.DeleteOlderThan(ctx, time.Date(2026, 3, 20, 9, 0, 1, 0, time.UTC)); err != nil {
		t.Fatalf("DeleteOlderThan returned error: %v", err)
	}

	traces, err := store.Query(ctx, QueryFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(traces) != 2 {
		t.Fatalf("expected 2 traces after age pruning, got %d", len(traces))
	}

	if err := store.TrimToCount(ctx, 1); err != nil {
		t.Fatalf("TrimToCount returned error: %v", err)
	}
	traces, err = store.Query(ctx, QueryFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace after count pruning, got %d", len(traces))
	}

	rule, err := store.UpsertAlertRule(ctx, AlertRule{
		ID:            "rule-1",
		Name:          "Error budget",
		RuleType:      "error_rate",
		Threshold:     5,
		WindowMinutes: 15,
		ServerName:    "demo-server",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("UpsertAlertRule returned error: %v", err)
	}
	if rule.ID != "rule-1" {
		t.Fatalf("id = %q", rule.ID)
	}

	rules, err := store.ListAlertRules(ctx)
	if err != nil {
		t.Fatalf("ListAlertRules returned error: %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "Error budget" {
		t.Fatalf("unexpected alert rules: %+v", rules)
	}

	if err := store.DeleteAlertRule(ctx, "rule-1"); err != nil {
		t.Fatalf("DeleteAlertRule returned error: %v", err)
	}
	rules, err = store.ListAlertRules(ctx)
	if err != nil {
		t.Fatalf("ListAlertRules returned error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected no alert rules after deletion, got %+v", rules)
	}
}
