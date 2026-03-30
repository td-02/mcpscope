package cmd

import "testing"

func TestValidatePort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{name: "valid default", port: 4444},
		{name: "valid low bound", port: 1},
		{name: "valid high bound", port: 65535},
		{name: "invalid zero", port: 0, wantErr: true},
		{name: "invalid negative", port: -1, wantErr: true},
		{name: "invalid too high", port: 65536, wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validatePort(tc.port)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for port %d", tc.port)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for port %d: %v", tc.port, err)
			}
		})
	}
}

func TestValidateTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		transport string
		want      string
		wantErr   bool
	}{
		{name: "stdio", transport: "stdio", want: "stdio"},
		{name: "http uppercase", transport: "HTTP", want: "http"},
		{name: "trimmed", transport: " stdio ", want: "stdio"},
		{name: "invalid", transport: "grpc", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateTransport(tc.transport)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for transport %q", tc.transport)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("validateTransport(%q) = %q, want %q", tc.transport, got, tc.want)
			}
		})
	}
}

func TestResolveProxyTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		server      string
		upstreamURL string
		transport   string
		args        []string
		wantCommand []string
		wantURL     string
		wantErr     bool
	}{
		{
			name:        "stdio command via args",
			transport:   "stdio",
			args:        []string{"uv", "run", "server.py"},
			wantCommand: []string{"uv", "run", "server.py"},
		},
		{
			name:        "http upstream url",
			transport:   "http",
			upstreamURL: "http://127.0.0.1:9000",
			wantURL:     "http://127.0.0.1:9000",
		},
		{
			name:      "http url through server flag",
			server:    "http://127.0.0.1:9000",
			transport: "http",
			wantURL:   "http://127.0.0.1:9000",
		},
		{
			name:      "conflicting server and args",
			server:    "server.exe",
			transport: "stdio",
			args:      []string{"node", "server.js"},
			wantErr:   true,
		},
		{
			name:        "upstream with stdio",
			upstreamURL: "http://127.0.0.1:9000",
			transport:   "stdio",
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveProxyTarget(tc.server, tc.upstreamURL, tc.transport, tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.upstreamURL != tc.wantURL {
				t.Fatalf("upstreamURL = %q, want %q", got.upstreamURL, tc.wantURL)
			}
			if len(got.command) != len(tc.wantCommand) {
				t.Fatalf("command = %v, want %v", got.command, tc.wantCommand)
			}
			for i := range got.command {
				if got.command[i] != tc.wantCommand[i] {
					t.Fatalf("command = %v, want %v", got.command, tc.wantCommand)
				}
			}
		})
	}
}

func TestParseRetentionDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "168h", want: "168h0m0s"},
		{raw: "0", want: "0s"},
		{raw: "bad", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()

			got, err := parseRetentionDuration(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Fatalf("duration = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNormalizeKeys(t *testing.T) {
	t.Parallel()

	got := normalizeKeys([]string{" token ", "TOKEN", "", "secret"})
	if len(got) != 2 {
		t.Fatalf("normalizeKeys length = %d, want 2", len(got))
	}
	if got[0] != "token" || got[1] != "secret" {
		t.Fatalf("normalizeKeys = %v", got)
	}
}
