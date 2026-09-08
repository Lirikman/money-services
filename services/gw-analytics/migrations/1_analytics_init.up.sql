CREATE TABLE IF NOT EXISTS analytics.transaction_events
(
    transaction_id String,
    user_id String,
    operation LowCardinality(String),
    created_at DateTime64(3, 'UTC'),
    received_at DateTime64(3, 'UTC'),
    latency_ms Int64,
    status LowCardinality(String) DEFAULT '',
    retry_count UInt16 DEFAULT 0,
    error String DEFAULT '',
    version UInt64
)
ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(created_at)
ORDER BY (transaction_id, created_at);