CREATE TABLE IF NOT EXISTS analytics.transaction_events
(
    transaction_id String,
    user_id String,
    operation LowCardinality(String),
    amount Float64,
    currency LowCardinality(String),
    created_at DateTime64(3, 'UTC'),
    received_at DateTime64(3, 'UTC'),
    latency_ms Int64 MATERIALIZED dateDiff('ms', created_at, received_at),
    status LowCardinality(String) DEFAULT '',
    retry_count UInt16 DEFAULT 0,
    error String DEFAULT '',
    version UInt64
)
ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(created_at)
ORDER BY (transaction_id, created_at);


CREATE TABLE IF NOT EXISTS analytics.transaction_aggregates
(
    period_start DateTime('UTC'),
    period_type LowCardinality(String),
    operation LowCardinality(String),
    status LowCardinality(String),
    events UInt64,
    errors UInt64,
    retries UInt64,
    latency_sum_ms UInt64,
    latency_max_ms UInt64
    version UInt64
)
ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(period_start)
ORDER BY (
    period_type,
    period_start,
    operation,
    status
);