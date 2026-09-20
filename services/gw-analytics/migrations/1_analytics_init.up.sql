CREATE TABLE IF NOT EXISTS transaction_events
(
    transaction_id String,
    user_id        String,
    operation      LowCardinality(String),
    status         LowCardinality(String),
    created_at     DateTime64(3, 'UTC'),
    received_at    DateTime64(3, 'UTC'),
    latency_ms     UInt64,
    retry_count    UInt32,
    error          String,
    version        UInt64
)
ENGINE = ReplacingMergeTree(version)
ORDER BY transaction_id;