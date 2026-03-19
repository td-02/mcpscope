CREATE TABLE IF NOT EXISTS traces (
    id TEXT PRIMARY KEY,
    trace_id TEXT NOT NULL,
    server_name TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT '',
    params_hash TEXT NOT NULL DEFAULT '',
    response_hash TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL,
    is_error INTEGER NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_traces_trace_id ON traces(trace_id);
CREATE INDEX IF NOT EXISTS idx_traces_server_name ON traces(server_name);
CREATE INDEX IF NOT EXISTS idx_traces_method ON traces(method);
CREATE INDEX IF NOT EXISTS idx_traces_created_at ON traces(created_at);
