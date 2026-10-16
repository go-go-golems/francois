-- FRANCOIS-007: scoped personal API tokens and token attribution in the audit log.

-- Existing tokens keep exactly what they could do before: the studio API.
ALTER TABLE api_tokens ADD COLUMN scopes text[] NOT NULL DEFAULT '{studio}';
-- New tokens must state their scopes.
ALTER TABLE api_tokens ALTER COLUMN scopes DROP DEFAULT;

-- The token a change was made through; NULL for browser sessions and the CLI.
ALTER TABLE admin_audit ADD COLUMN token_id text;
CREATE INDEX admin_audit_actor ON admin_audit (actor_id, created_at DESC);
CREATE INDEX admin_audit_subject ON admin_audit (subject, created_at DESC);
