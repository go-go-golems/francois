-- FRANCOIS-019: pending CLI device logins. A row lives for ten minutes and is
-- deleted when the CLI claims its token. Only hashes of both codes are stored;
-- the token itself is minted at claim time into api_tokens.
CREATE TABLE device_authorizations (
    device_code_hash bytea PRIMARY KEY,
    user_code_hash bytea NOT NULL UNIQUE,
    portal_origin text NOT NULL,
    client_name text NOT NULL,
    requested_scopes text[] NOT NULL,
    requested_days integer NOT NULL,
    requester_address text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'denied')),
    user_id text REFERENCES users(id),
    approved_scopes text[],
    approved_days integer,
    interval_seconds integer NOT NULL,
    last_polled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX device_authorizations_expiry ON device_authorizations (expires_at);
