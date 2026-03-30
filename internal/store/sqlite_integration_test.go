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
		Environment:     "prod",
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
	if got.Environment != trace.Environment {
		t.Fatalf("environment = %q, want %q", got.Environment, trace.Environment)
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

	filtered, err := store.Query(ctx, QueryFilter{TraceID: trace.TraceID, Environment: trace.Environment, Limit: 1})
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
			Environment:     "prod",
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
		Environment:   "prod",
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
	if rules[0].Environment != "prod" {
		t.Fatalf("expected prod environment, got %+v", rules[0])
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

func TestSQLiteStoreAlertEventsAndStats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "traces.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite returned error: %v", err)
	}
	defer store.Close()

	base := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	traces := []Trace{
		{
			ID: "trace-a", TraceID: "corr-a", Environment: "prod", ServerName: "alpha", Method: "tools/call",
			ParamsHash: "a", ParamsPayload: `{}`, ResponseHash: "a", ResponsePayload: `{}`,
			LatencyMs: 20, CreatedAt: base.Add(-4 * time.Minute),
		},
		{
			ID: "trace-b", TraceID: "corr-b", Environment: "prod", ServerName: "alpha", Method: "tools/call",
			ParamsHash: "b", ParamsPayload: `{}`, ResponseHash: "b", ResponsePayload: `{}`,
			LatencyMs: 100, IsError: true, ErrorMessage: "boom", CreatedAt: base.Add(-3 * time.Minute),
		},
		{
			ID: "trace-c", TraceID: "corr-c", Environment: "prod", ServerName: "alpha", Method: "tools/call",
			ParamsHash: "c", ParamsPayload: `{}`, ResponseHash: "c", ResponsePayload: `{}`,
			LatencyMs: 250, CreatedAt: base.Add(-2 * time.Minute),
		},
		{
			ID: "trace-d", TraceID: "corr-d", Environment: "stage", ServerName: "alpha", Method: "tools/call",
			ParamsHash: "d", ParamsPayload: `{}`, ResponseHash: "d", ResponsePayload: `{}`,
			LatencyMs: 5, CreatedAt: base.Add(-1 * time.Minute),
		},
	}
	for _, trace := range traces {
		if err := store.Insert(ctx, trace); err != nil {
			t.Fatalf("Insert returned error: %v", err)
		}
	}

	start := base.Add(-10 * time.Minute)
	latency, err := store.QueryLatencyStats(ctx, QueryFilter{
		Environment:  "prod",
		ServerName:   "alpha",
		Method:       "tools/call",
		CreatedAfter: &start,
	})
	if err != nil {
		t.Fatalf("QueryLatencyStats returned error: %v", err)
	}
	if len(latency) != 1 {
		t.Fatalf("expected 1 latency row, got %+v", latency)
	}
	if latency[0].Count != 3 || latency[0].P95Ms != 250 {
		t.Fatalf("unexpected latency stats: %+v", latency[0])
	}

	errors, err := store.QueryErrorStats(ctx, QueryFilter{
		Environment:  "prod",
		Method:       "tools/call",
		CreatedAfter: &start,
	})
	if err != nil {
		t.Fatalf("QueryErrorStats returned error: %v", err)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error row, got %+v", errors)
	}
	if errors[0].ErrorCount != 1 || errors[0].Count != 3 {
		t.Fatalf("unexpected error stats: %+v", errors[0])
	}

	event := AlertEvent{
		ID:             "event-1",
		RuleID:         "rule-1",
		Environment:    "prod",
		RuleName:       "Prod latency",
		Status:         "firing",
		PreviousStatus: "ok",
		CurrentValue:   250,
		Threshold:      100,
		SampleCount:    3,
		Notification:   "https://example.invalid/webhook",
		DeliveryStatus: "failed",
		DeliveryError:  "timeout",
		CreatedAt:      base,
	}
	if err := store.InsertAlertEvent(ctx, event); err != nil {
		t.Fatalf("InsertAlertEvent returned error: %v", err)
	}

	events, err := store.ListAlertEvents(ctx, "prod", 10)
	if err != nil {
		t.Fatalf("ListAlertEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].RuleName != "Prod latency" {
		t.Fatalf("unexpected events: %+v", events)
	}

	latest, err := store.LatestAlertEvent(ctx, "prod", "rule-1")
	if err != nil {
		t.Fatalf("LatestAlertEvent returned error: %v", err)
	}
	if latest == nil || latest.Status != "firing" {
		t.Fatalf("unexpected latest event: %+v", latest)
	}
}
