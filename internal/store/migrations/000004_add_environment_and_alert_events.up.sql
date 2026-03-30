ALTER TABLE traces ADD COLUMN environment TEXT NOT NULL DEFAULT 'default';
ALTER TABLE alert_rules ADD COLUMN environment TEXT NOT NULL DEFAULT 'default';

CREATE INDEX IF NOT EXISTS idx_traces_environment ON traces(environment);
CREATE INDEX IF NOT EXISTS idx_alert_rules_environment ON alert_rules(environment);

CREATE TABLE IF NOT EXISTS alert_events (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    rule_name TEXT NOT NULL,
    status TEXT NOT NULL,
    previous_status TEXT NOT NULL DEFAULT '',
    current_value REAL NOT NULL,
    threshold REAL NOT NULL,
    sample_count INTEGER NOT NULL,
    notification TEXT NOT NULL DEFAULT '',
    delivery_status TEXT NOT NULL DEFAULT '',
    delivery_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_events_rule_id ON alert_events(rule_id);
CREATE INDEX IF NOT EXISTS idx_alert_events_environment ON alert_events(environment);
CREATE INDEX IF NOT EXISTS idx_alert_events_created_at ON alert_events(created_at);
