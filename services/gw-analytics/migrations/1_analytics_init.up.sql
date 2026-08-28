CREATE TABLE IF NOT EXISTS analytics.transaction_events
(
    transaction_id String,
    user_id String,

    operation LowCardinality(String),
    status LowCardinality(String),

    amount Float64,
    currency LowCardinality(String),

    created_at DateTime64(3, 'UTC'),
    received_at DateTime64(3, 'UTC'),

    latency_ms UInt64,

    retry_count UInt32,

    error String,

    version UInt64
)
ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(created_at)
ORDER BY transaction_id;


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
    latency_max_ms UInt64,

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