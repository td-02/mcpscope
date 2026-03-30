<p align="center">
  <img src="https://via.placeholder.com/120?text=mcpscope" alt="mcpscope" width="120">
</p>

<h1 align="center">mcpscope</h1>

<p align="center">The open source observability layer for MCP servers</p>

<p align="center">
  <a href="https://github.com/td-02/mcp-observer/actions/workflows/ci.yml"><img src="https://github.com/td-02/mcp-observer/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a>
  <a href="/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.22-00ADD8?logo=go" alt="Go 1.22"></a>
  <a href="https://github.com/td-02/mcp-observer/releases"><img src="https://img.shields.io/github/v/release/td-02/mcp-observer" alt="Latest Release"></a>
</p>

<p align="center">
  <img src="demo/mcp_observer_demo.gif" alt="mcpscope demo">
</p>

## Why mcpscope

Agents increasingly call MCP tools in production, but when something fails the execution path often disappears into stderr noise, missing logs, or vendor black boxes. mcpscope is meant to be the open source layer that captures MCP request and response traffic, keeps it queryable, and turns it into debugging, alerting, and compliance-ready operational history.

## Features

- Transparent proxy for stdio and HTTP MCP transports
- Live web dashboard with trace feed, latency percentiles, error rates, alert state, and alert event history
- Environment scoping across traces, stats, alerts, export, and replay workflows
- Bearer token protection for dashboard APIs and SSE streams
- Retention policies and paginated trace browsing
- Payload redaction before logs, storage, and UI rendering
- Built-in latency P95 and error-rate alerts with webhook delivery
- SQLite persistence with SQL-backed latency and error aggregations
- Trace export and replay for debugging, CI, and fixture generation
- Schema snapshot and diff CLI for MCP compatibility checks
- OpenTelemetry export for external tracing systems

## Quickstart

```bash
go install github.com/YOUR_USERNAME/mcpscope@latest
```

```bash
mcpscope proxy --server ./your-mcp-server --db traces.db
```

```bash
mcpscope proxy --transport http --upstream-url http://127.0.0.1:8080 --db traces.db
```

```bash
mcpscope proxy --transport stdio -- uv run server.py
```

```bash
mcpscope proxy --retain-for 72h --max-traces 10000 --redact-key authorization --redact-key api_key -- uv run server.py
```

```bash
mcpscope proxy --config ./mcpscope.example.json -- uv run server.py
```

Open the dashboard at `http://localhost:4444`.

If `authToken` or `--auth-token` is set, the dashboard requires that token for `/api/*` and `/events`. The built-in UI exposes a token field and stores it locally in the browser.

## Schema diff in CI

```bash
mcpscope snapshot --server ./your-mcp-server --output baseline.json
```

```bash
mcpscope snapshot -- uv run server.py --output baseline.json
```

```bash
git add baseline.json && git commit -m "chore: add MCP baseline snapshot"
```

```bash
mcpscope snapshot --server ./your-mcp-server --output current.json
mcpscope diff baseline.json current.json --exit-code
```

See `examples/github-actions/mcp-schema-check.yml`.

## Export and replay

```bash
mcpscope export --config ./mcpscope.example.json --output traces.json --limit 200
```

```bash
mcpscope replay --input traces.json --transport http --server http://127.0.0.1:8080
```

```bash
mcpscope replay --input traces.json -- uv run server.py
```

Use export plus replay for regression debugging, smoke tests, and captured MCP fixtures.

## Configuration

| Flag | Default | Description |
| --- | --- | --- |
| `--config` | none | Path to a JSON config file for proxy or export workflows. |
| `--server` | none | Path to the target MCP server binary. |
| `--upstream-url` | none | HTTP URL for an already running MCP server with `proxy --transport http`. |
| `--port` | `4444` | Port used by the built-in dashboard and HTTP proxy mode. |
| `--db` | `mcpscope.db` | SQLite database path used for persisted traces. |
| `--transport` | `stdio` | Proxy transport mode: `stdio` or `http`. |
| `--otel` | `false` | Enables OpenTelemetry span export. |
| `--retain-for` | `168h` | Age-based trace retention. Use `0` to disable. |
| `--max-traces` | `5000` | Count-based retention limit. Use `0` to disable. |
| `--redact-key` | common secret fields | JSON field name to redact before logs, storage, and dashboard output. Repeatable. |
| `--environment` | `default` | Logical environment for traces, alerts, stats, export, and replay. |
| `--auth-token` | none | Bearer token required for dashboard APIs and SSE when set. |
| `--notify-webhook` | none | Webhook URL that receives alert transition events. Repeatable. |

Both `proxy` and `snapshot` accept launch commands after `--`, which is the preferred way to run servers that need arguments such as `uv run server.py` or `node server.js`.

### JSON config file

`mcpscope proxy` and `mcpscope export` accept `--config path/to/mcpscope.json`. A baseline example is included in [`mcpscope.example.json`](mcpscope.example.json).

```json
{
  "environment": "prod",
  "authToken": "replace-me",
  "notification": {
    "webhookUrls": ["https://example.com/mcpscope-alerts"]
  },
  "proxy": {
    "db": "/data/mcpscope.db",
    "port": 4444,
    "transport": "stdio",
    "retainFor": "168h",
    "maxTraces": 5000,
    "redactKeys": ["authorization", "token", "secret"],
    "otel": false
  }
}
```

Flags override config file values.

## Architecture

```text
                    +------------------+
                    |     AI Agent     |
                    +------------------+
                              |
                              v
                    +------------------+
                    | mcpscope proxy   |
                    +------------------+
                     |       |       |
                     |       |       +--------------------+
                     |       |                            |
                     |       v                            v
                     |  +-----------+              +-------------+
                     |  | SQLite    |              | Web         |
                     |  | store     |              | dashboard   |
                     |  +-----------+              +-------------+
                     |       |                            |
                     |       +----------------------------+
                     |                    |
                     v                    v
               +-------------+      +-------------+
               | OTEL        |      | Webhooks    |
               | exporter    |      | / alerts    |
               +-------------+      +-------------+
                              |
                              v
                    +------------------+
                    |   MCP Server(s)  |
                    +------------------+
```

## Docker deployment

The provided `docker-compose.yml` expects:

- `MCP_SERVER_PATH` pointing to the server binary mounted into the container at `/mcp-server`
- `MCPSCOPE_CONFIG_PATH` pointing to a JSON config file mounted at `/config/mcpscope.json`

Example:

```bash
$env:MCP_SERVER_PATH="C:\path\to\linux\mcp-server"
$env:MCPSCOPE_CONFIG_PATH=".\mcpscope.example.json"
docker compose up --build
```

For containerized deployments:

- Mount `/data` for SQLite persistence.
- Mount `/config/mcpscope.json` and start with `mcpscope --config /config/mcpscope.json proxy -- /mcp-server`.
- Set `OTEL_EXPORTER_OTLP_ENDPOINT` when you want OTEL spans forwarded.

## Roadmap

- [ ] Per-team budget enforcement
- [ ] Mock server mode from captured traces
- [ ] Audit log export (CSV and JSON)
- [ ] Slack and PagerDuty alert routing
- [ ] Hosted cloud version

## Contributing

Review [CHANGELOG.md](CHANGELOG.md) before opening substantial feature work so versioned behavior stays coherent. Pull requests are welcome, especially when they include tests and docs updates alongside the code change. For bug reports, feature ideas, MCP integration requests, documentation fixes, and usage questions, start with the templates in [.github/ISSUE_TEMPLATE/](.github/ISSUE_TEMPLATE/).

## License

MIT. See [LICENSE](LICENSE).
