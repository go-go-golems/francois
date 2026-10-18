-- Replace the two internal pending-email workflows. Existing accounts and
-- sessions remain; old short-lived links are intentionally not migrated.
DROP TABLE email_signins;
DROP TABLE email_registrations;

CREATE TABLE email_links (
 flow_hash bytea PRIMARY KEY,
 binding_hash bytea NOT NULL,
 issuer text NOT NULL,
 subject text NOT NULL,
 email text NOT NULL,
 role text NOT NULL,
 portal_origin text NOT NULL,
 return_path text NOT NULL,
 ready boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL
);
CREATE INDEX email_links_identity ON email_links(issuer,subject);
CREATE INDEX email_links_expiry ON email_links(expires_at);
