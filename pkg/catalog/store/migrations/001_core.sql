-- Accounts, sessions and audit.
CREATE TABLE users (
    id text PRIMARY KEY,
    issuer text NOT NULL,
    subject text NOT NULL,
    email text NOT NULL DEFAULT '',
    email_verified boolean NOT NULL DEFAULT false,
    display_name text NOT NULL DEFAULT '',
    role text NOT NULL DEFAULT 'subscriber' CHECK (role IN ('subscriber', 'admin')),
    disabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz,
    UNIQUE (issuer, subject)
);
CREATE INDEX users_email ON users (lower(email));

CREATE TABLE login_flows (
    state_hash bytea PRIMARY KEY,
    binding_hash bytea NOT NULL,
    nonce text NOT NULL,
    verifier text NOT NULL,
    return_path text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id),
    csrf_token text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE access_grants (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id),
    until timestamptz NOT NULL,
    reason text NOT NULL,
    granted_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
CREATE INDEX access_grants_user ON access_grants (user_id);

CREATE TABLE admin_audit (
    id bigserial PRIMARY KEY,
    actor_id text,
    action text NOT NULL,
    subject text NOT NULL,
    summary jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_time ON admin_audit (created_at DESC);

-- Validated nonsecret platform settings; exactly one active revision.
CREATE TABLE platform_settings (
    revision bigserial PRIMARY KEY,
    document jsonb NOT NULL,
    actor_id text,
    activated_at timestamptz NOT NULL DEFAULT now()
);

-- Curated catalog and immutable releases.
CREATE TABLE catalog_artifacts (
    id text PRIMARY KEY,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    tags text[] NOT NULL DEFAULT '{}',
    visibility text NOT NULL DEFAULT 'listed' CHECK (visibility IN ('listed', 'hidden')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE releases (
    id text PRIMARY KEY,
    artifact_id text NOT NULL REFERENCES catalog_artifacts(id),
    version text NOT NULL,
    changelog text NOT NULL DEFAULT '',
    manifest jsonb NOT NULL,
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'published', 'withdrawn')),
    free_sample boolean NOT NULL DEFAULT false,
    created_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    published_by text REFERENCES users(id),
    withdrawn_at timestamptz,
    UNIQUE (artifact_id, version)
);

CREATE TABLE release_files (
    id text PRIMARY KEY,
    release_id text NOT NULL REFERENCES releases(id),
    role text NOT NULL CHECK (role IN ('html', 'manifest')),
    name text NOT NULL,
    media_type text NOT NULL,
    size bigint NOT NULL,
    sha256 text NOT NULL,
    storage_key text NOT NULL,
    UNIQUE (release_id, role)
);

-- Hosted application instances and their durable state.
CREATE TABLE instances (
    id text PRIMARY KEY,
    hostname text NOT NULL UNIQUE,
    owner_user_id text NOT NULL REFERENCES users(id),
    release_id text NOT NULL REFERENCES releases(id),
    title text NOT NULL,
    kind text NOT NULL DEFAULT 'workspace' CHECK (kind IN ('workspace', 'preview')),
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'suspended')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX instances_owner ON instances (owner_user_id);

CREATE TABLE instance_members (
    instance_id text NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES users(id),
    role text NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    added_by text REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (instance_id, user_id)
);
CREATE INDEX instance_members_user ON instance_members (user_id);

CREATE TABLE instance_documents (
    instance_id text NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    path text NOT NULL,
    collection text NOT NULL,
    body jsonb NOT NULL,
    size integer NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    updated_by text,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (instance_id, path)
);
CREATE INDEX instance_documents_collection ON instance_documents (instance_id, collection);

CREATE TABLE instance_leases (
    instance_id text NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    path text NOT NULL,
    holder text NOT NULL,
    expires_at timestamptz NOT NULL,
    data jsonb,
    PRIMARY KEY (instance_id, path)
);

CREATE TABLE instance_assets (
    id text PRIMARY KEY,
    instance_id text NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    owner_user_id text NOT NULL REFERENCES users(id),
    media_type text NOT NULL,
    bytes bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX instance_assets_instance ON instance_assets (instance_id);

CREATE TABLE instance_threads (
    id text PRIMARY KEY,
    instance_id text NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    anchor jsonb NOT NULL,
    resolved boolean NOT NULL DEFAULT false,
    created_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE instance_comments (
    id text PRIMARY KEY,
    thread_id text NOT NULL REFERENCES instance_threads(id) ON DELETE CASCADE,
    author_id text NOT NULL REFERENCES users(id),
    text text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Browser-bound launch exchange and instance-scoped runtime grants.
CREATE TABLE launch_codes (
    code_hash bytea PRIMARY KEY,
    session_hash bytea NOT NULL,
    user_id text NOT NULL REFERENCES users(id),
    instance_id text NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    origin text NOT NULL,
    challenge text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);

CREATE TABLE runtime_grants (
    token_hash bytea PRIMARY KEY,
    session_hash bytea NOT NULL,
    user_id text NOT NULL REFERENCES users(id),
    instance_id text NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX runtime_grants_user ON runtime_grants (user_id);
CREATE INDEX runtime_grants_instance ON runtime_grants (instance_id);
