# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

- Added trace search plus `created_after` and `created_before` filtering in the dashboard and trace APIs.
- Added alert rule editing and enable/disable controls in the dashboard.
- Tightened trace timestamp handling so retention and time-range filters behave consistently.

## v0.1.0

- Added a transparent MCP proxy for stdio and HTTP transport with unchanged JSON-RPC forwarding.
- Added JSON-RPC interception with structured stderr logging, trace IDs, timestamps, and request/response parsing.
- Added a SQLite-backed trace store with migrations and queryable trace history.
- Added OpenTelemetry export with OTLP gRPC support and Jaeger example wiring.
- Added Docker and Docker Compose packaging for local proxy deployment and observability demos.
- Added an embedded web dashboard with live traces, latency percentile views, error views, and SSE updates.
- Added the `snapshot` command for capturing MCP tool schemas from stdio or HTTP servers.
- Added the `diff` command for schema comparisons, CI-friendly breaking-change detection, and JSON output.
- Added GitHub Actions CI and an example pull-request schema check workflow.
