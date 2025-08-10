CREATE TABLE IF NOT EXISTS TOKENS (
    hash bytea PRIMARY KEY,
    scope text NOT NULL,
    expiry timestamptz(0) NOT NULL,
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE
)