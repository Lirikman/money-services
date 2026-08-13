CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL CHECK (length(trim(username)) > 0),
    email VARCHAR(254) UNIQUE NOT NULL CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    currency VARCHAR(3) NOT NULL CHECK (currency IN ('RUB', 'USD', 'EUR')),
    balance DECIMAL(15, 2) NOT NULL DEFAULT 0.00,
    updated_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT unique_user_currency UNIQUE (user_id, currency)
);

CREATE INDEX idx_wallets_user_currency ON wallets(user_id, currency);