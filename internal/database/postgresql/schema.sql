CREATE TABLE IF NOT EXISTS clients (
    client_id TEXT PRIMARY KEY,
    api_key TEXT NOT NULL,
    capacity INT NOT NULL,
    CHECK(capacity > 0),
    rate_per_sec INT NOT NULL,
    CHECK(rate_per_sec > 0),
    tokens INT NOT NULL,
    CHECK(tokens >= 0),
    last_refill TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX ON clients(api_key);