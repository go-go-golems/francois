-- A reserved provider identity and its current browser-bound signup attempt.
-- This is not an authenticated local user. Keep the reservation after claims
-- so retries recover the same identity after partial provider-side success.
CREATE TABLE email_registrations (
    subject text PRIMARY KEY,
    issuer text NOT NULL,
    email text NOT NULL,
    flow_hash bytea UNIQUE,
    binding_hash bytea,
    portal_origin text NOT NULL DEFAULT '',
    return_path text NOT NULL DEFAULT '/',
    provider_session_id text NOT NULL DEFAULT '',
    ready boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(issuer,email),
    CHECK(email=lower(email))
);
