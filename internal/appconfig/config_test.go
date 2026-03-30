package appconfig

import (
	"path/filepath"
	"testing"
	"time"

	"os"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "mcpscope.json")
	err := os.WriteFile(path, []byte(`{
  "environment": "prod",
  "authToken": "top-secret",
  "notification": {
    "webhookUrls": ["https://example.invalid/a"]
  },
  "proxy": {
    "db": "traces.db",
    "port": 5555,
    "transport": "http",
    "retainFor": "72h",
    "maxTraces": 123,
    "redactKeys": ["token"],
    "otel": true
  }
}`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Environment != "prod" || cfg.AuthToken != "top-secret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Proxy.Port != 5555 || cfg.Proxy.Transport != "http" || !cfg.Proxy.EnableOTEL {
		t.Fatalf("unexpected proxy config: %+v", cfg.Proxy)
	}
	if got := cfg.RetentionDuration(); got != 72*time.Hour {
		t.Fatalf("RetentionDuration = %s", got)
	}
}
