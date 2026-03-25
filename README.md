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

Agents increasingly call MCP tools in production, but when something fails the execution path often disappears into stderr noise, missing logs, or vendor black boxes. There is still no widely adopted open source observability layer purpose-built for MCP traffic, traces, schema drift, and live debugging. That gap becomes more urgent as teams prepare for auditability and operational transparency requirements ahead of the EU AI Act compliance deadline in August 2026.

## Features

- Transparent proxy — zero config, works with stdio and HTTP MCP transports
- Live web dashboard — tool call feed, P50/P95/P99 latency histograms, error rate timelines
- OpenTelemetry export — plugs into any existing Grafana or Jaeger setup
- SQLite trace store — local by default, Postgres-ready via swappable backend interface
- Schema snapshot + diff CLI — catch breaking tool changes before they reach production
- GitHub Actions example — blocks PRs on breaking MCP server schema changes

## Quickstart

```bash
go install github.com/YOUR_USERNAME/mcpscope@latest
```

```bash
mcpscope proxy --server ./your-mcp-server --db traces.db
```

```text
http://localhost:4444
```

## Schema diff in CI

```bash
mcpscope snapshot --server ./your-mcp-server --output baseline.json
```

```bash
git add baseline.json && git commit -m "chore: add MCP baseline snapshot"
```

```bash
# On PR:
mcpscope snapshot --server ./your-mcp-server --output current.json
mcpscope diff baseline.json current.json --exit-code
```

```text
examples/github-actions/mcp-schema-check.yml
```

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
                     |
                     v
               +-------------+
               | OTEL        |
               | exporter    |
               +-------------+
                              |
                              v
                    +------------------+
                    |   MCP Server(s)  |
                    +------------------+
```

## Configuration

| Flag | Env var | Default | Description |
| --- | --- | --- | --- |
| `--server` |  | none | Path to the target MCP server binary or HTTP URL. |
| `--port` |  | `4444` | Port used by the built-in HTTP server and HTTP proxy mode. |
| `--db` |  | `mcpscope.db` | SQLite database path used for persisted traces. |
| `--transport` |  | `stdio` | MCP transport mode for the proxy: `stdio` or `http`. |
| `--otel` |  | `false` | Explicitly enables OpenTelemetry span export. |
|  | `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP gRPC endpoint for trace export; when unset, mcpscope falls back to a no-op exporter silently. |

## Roadmap

- [ ] Per-team budget enforcement
- [ ] Replay and mock server for integration testing
- [ ] Audit log export (CSV and JSON)
- [ ] Slack and PagerDuty alerting
- [ ] Hosted cloud version

## Contributing

Review [CHANGELOG.md](CHANGELOG.md) before opening substantial feature work so versioned behavior stays coherent. Pull requests are welcome, especially when they include tests and docs updates alongside the code change. For bug reports, feature ideas, MCP integration requests, documentation fixes, and usage questions, start with the templates in [.github/ISSUE_TEMPLATE/](.github/ISSUE_TEMPLATE/).

## License

MIT. See [LICENSE](LICENSE).
