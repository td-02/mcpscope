package store

import (
	"context"
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
		ID:           "550e8400-e29b-41d4-a716-446655440000",
		TraceID:      "8f14e45f-ea4c-4d57-8b55-6e5d7f1db7b1",
		ServerName:   "demo-server",
		Method:       "tools/call",
		ParamsHash:   "params-hash",
		ResponseHash: "response-hash",
		LatencyMs:    25,
		IsError:      true,
		ErrorMessage: "boom",
		CreatedAt:    createdAt,
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
