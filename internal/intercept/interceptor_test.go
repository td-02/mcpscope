package intercept

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestParseMessageRequest(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"ping"}}`)
	parsed, err := ParseMessage(payload)
	if err != nil {
		t.Fatalf("ParseMessage returned error: %v", err)
	}

	if got := string(parsed.ID); got != "7" {
		t.Fatalf("id = %s, want 7", got)
	}
	if parsed.Method != "tools/call" {
		t.Fatalf("method = %q, want tools/call", parsed.Method)
	}
	if got := string(parsed.Params); got != `{"name":"ping"}` {
		t.Fatalf("params = %s", got)
	}
	if parsed.Result != nil {
		t.Fatalf("expected nil result, got %s", string(parsed.Result))
	}
	if parsed.Error != nil {
		t.Fatalf("expected nil error, got %s", string(parsed.Error))
	}
}

func TestParseMessageResponse(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"jsonrpc":"2.0","id":"abc","result":{"ok":true}}`)
	parsed, err := ParseMessage(payload)
	if err != nil {
		t.Fatalf("ParseMessage returned error: %v", err)
	}

	if got := string(parsed.ID); got != `"abc"` {
		t.Fatalf("id = %s, want \"abc\"", got)
	}
	if parsed.Method != "" {
		t.Fatalf("method = %q, want empty", parsed.Method)
	}
	if got := string(parsed.Result); got != `{"ok":true}` {
		t.Fatalf("result = %s", got)
	}
}

func TestParseMessageErrorResponse(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"missing"}}`)
	parsed, err := ParseMessage(payload)
	if err != nil {
		t.Fatalf("ParseMessage returned error: %v", err)
	}

	if got := string(parsed.Error); got != `{"code":-32601,"message":"missing"}` {
		t.Fatalf("error = %s", got)
	}
	if parsed.Result != nil {
		t.Fatalf("expected nil result, got %s", string(parsed.Result))
	}
}

func TestEmitLogIncludesStructuredFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	receivedAt := time.Unix(0, 10)
	sentAt := time.Unix(0, 20)

	event := Capture("stdio", "client_to_server", receivedAt, sentAt, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"x":1}}`))

	err := EmitLog(&buf, event)
	if err != nil {
		t.Fatalf("EmitLog returned error: %v", err)
	}

	var entry Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("unmarshal log entry: %v", err)
	}

	if entry.TraceID == "" {
		t.Fatalf("expected trace id")
	}
	if entry.Transport != "stdio" {
		t.Fatalf("transport = %q", entry.Transport)
	}
	if entry.Direction != "client_to_server" {
		t.Fatalf("direction = %q", entry.Direction)
	}
	if entry.ReceivedAtUnixN != 10 {
		t.Fatalf("received_at_unix_nano = %d", entry.ReceivedAtUnixN)
	}
	if entry.SentAtUnixN != 20 {
		t.Fatalf("sent_at_unix_nano = %d", entry.SentAtUnixN)
	}
	if entry.LatencyMs != 0 {
		t.Fatalf("latency_ms = %d, want 0", entry.LatencyMs)
	}
	if got := string(entry.ID); got != "1" {
		t.Fatalf("id = %s", got)
	}
	if entry.Method != "ping" {
		t.Fatalf("method = %q", entry.Method)
	}
	if got := string(entry.Params); got != `{"x":1}` {
		t.Fatalf("params = %s", got)
	}
	if entry.ParamsHash == "" {
		t.Fatalf("expected params hash to be populated")
	}
}

func TestEmitLogIncludesParseErrorForInvalidJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	event := Capture("http", "server_to_client", time.Unix(0, 1), time.Unix(0, 2), []byte(`not-json`))
	err := EmitLog(&buf, event)
	if err != nil {
		t.Fatalf("EmitLog returned error: %v", err)
	}

	var entry Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("unmarshal log entry: %v", err)
	}

	if entry.ParseError == "" {
		t.Fatalf("expected parse_error to be populated")
	}
	if !entry.IsError {
		t.Fatalf("expected is_error to be true")
	}
}

func TestCaptureExtractsErrorMessageAndResponseHash(t *testing.T) {
	t.Parallel()

	event := Capture("stdio", "server_to_client", time.Unix(0, 100), time.Unix(0, 2_000_000), []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"missing"}}`))

	if !event.IsError {
		t.Fatalf("expected is_error to be true")
	}
	if event.ErrorMessage != "missing" {
		t.Fatalf("error_message = %q, want missing", event.ErrorMessage)
	}
	if event.ResponseHash == "" {
		t.Fatalf("expected response_hash to be populated")
	}
	if event.LatencyMs != 1 {
		t.Fatalf("latency_ms = %d, want 1", event.LatencyMs)
	}
}
