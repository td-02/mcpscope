# mcpscope

`mcpscope` is a lightweight MCP proxy that sits in front of any MCP server, forwards traffic unchanged, and gives you visibility that most MCP setups lack by default: structured interception logs, SQLite-backed trace persistence, and optional OpenTelemetry export to backends like Jaeger.

## Prerequisites

- Go 1.22 or Docker
- An MCP server binary you want to wrap
- Optional: Jaeger or another OTLP-compatible collector if you want exported spans

## 5-Minute Quickstart

Wrap any MCP server with `mcpscope` in three commands:

```powershell
git clone https://github.com/td-02/mcpscope.git
cd mcpscope
go run . proxy --server "C:\path\to\your-mcp-server.exe"
```

By default this starts the proxy in stdio mode, writes traces into `mcpscope.db`, and emits structured interception logs to `stderr`. Add `--otel` and set `OTEL_EXPORTER_OTLP_ENDPOINT` if you want OTEL spans as well.

## Architecture

```text
Caller / MCP Client
        |
        v
+-------------------+
|     mcpscope      |
|-------------------|
| stdio/http proxy  |
| JSON-RPC parsing  |
| stderr log output |
| SQLite trace store|
| OTEL span export  |
+-------------------+
        |
        v
  Target MCP Server
```

## Configuration

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `--server` | flag | none | Path to the MCP server binary that `mcpscope` launches as a subprocess. |
| `--port` | flag | `4444` | Proxy listen port. Used directly for HTTP transport and reserved for stdio mode compatibility. |
| `--transport` | flag | `stdio` | Proxy transport mode: `stdio` or `http`. |
| `--db` | flag | `mcpscope.db` | SQLite database path used for persisted trace storage. |
| `--otel` | flag | `false` | Enables OpenTelemetry export for intercepted MCP tool calls. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | env var | unset | OTLP gRPC endpoint for span export, for example `localhost:4317` or `jaeger:4317`. If unset, `--otel` falls back to a no-op exporter silently. |

## Docker

Build the container image:

```bash
docker build -t mcpscope:local .
```

Run with Docker Compose:

```bash
export MCP_SERVER_PATH=/absolute/path/to/linux-mcp-server
docker compose up --build
```

Enable Jaeger alongside the proxy:

```bash
export MCP_SERVER_PATH=/absolute/path/to/linux-mcp-server
export OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317
docker compose --profile observability up --build
```

Open Jaeger at `http://localhost:16686`.

## Contributing

1. Fork the repository and create a feature branch from `main`.
2. Keep changes focused and run `go mod tidy`, `go test ./...`, and `go build ./...` before opening a pull request.
3. Add or update tests whenever you change parsing, persistence, transport, or telemetry behavior.
4. Prefer small, reviewable commits with clear messages.
