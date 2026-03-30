package proxy

import (
	"encoding/json"
	"testing"
)

func TestPayloadRedactorRedactsNestedKeys(t *testing.T) {
	t.Parallel()

	redactor := newPayloadRedactor([]string{"token", "password"})
	raw := json.RawMessage(`{"token":"abc","nested":{"password":"secret","safe":"ok"}}`)
	got := redactor.Raw(raw)

	if string(got) != `{"nested":{"password":"[REDACTED]","safe":"ok"},"token":"[REDACTED]"}` {
		t.Fatalf("redacted payload = %s", string(got))
	}
}
