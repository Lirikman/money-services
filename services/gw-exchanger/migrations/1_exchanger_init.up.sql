CREATE TABLE IF NOT EXISTS exchange_rates (
    id SERIAL PRIMARY KEY,
    base_currency VARCHAR(3) NOT NULL,
    target_currency VARCHAR(3) NOT NULL,
    rate NUMERIC(18,8) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(base_currency, target_currency)
);