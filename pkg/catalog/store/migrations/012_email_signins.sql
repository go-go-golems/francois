-- A browser-bound pending attempt. Mailbox proof is generated and checked
-- by ZITADEL; no provider bearer token or email code is persisted here.
CREATE TABLE email_signins (
    flow_hash bytea PRIMARY KEY,
    user_id text NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    binding_hash bytea NOT NULL,
    issuer text NOT NULL,
    subject text NOT NULL,
    email_snapshot text NOT NULL,
    portal_origin text NOT NULL,
    return_path text NOT NULL,
    provider_session_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX email_signins_expiry ON email_signins(expires_at);
