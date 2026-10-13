-- The ID token from sign-in, used only as id_token_hint when signing out
-- through the identity provider (RP-initiated logout). Cleared on revoke.
ALTER TABLE sessions ADD COLUMN id_token text NOT NULL DEFAULT '';
