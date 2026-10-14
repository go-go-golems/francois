-- FRANCOIS-004: storefront content, creator and guest roles, plans on
-- complimentary grants, demo instances and personal API tokens.

ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('subscriber', 'creator', 'admin', 'guest'));

-- page is the published page document ({"body","cover","gallery"});
-- page_draft is the editor's unpublished copy. The remaining columns apply
-- immediately, like title and visibility.
ALTER TABLE catalog_artifacts
    ADD COLUMN page jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN page_draft jsonb,
    ADD COLUMN featured boolean NOT NULL DEFAULT false,
    ADD COLUMN sort_order integer NOT NULL DEFAULT 0,
    ADD COLUMN required_plan text NOT NULL DEFAULT '',
    ADD COLUMN demo_enabled boolean NOT NULL DEFAULT false;

-- Public images and videos, stored in the release blob store.
CREATE TABLE catalog_media (
    id text PRIMARY KEY,
    sha256 text NOT NULL,
    storage_key text NOT NULL,
    media_type text NOT NULL,
    size bigint NOT NULL,
    width integer NOT NULL DEFAULT 0,
    height integer NOT NULL DEFAULT 0,
    alt text NOT NULL DEFAULT '',
    uploaded_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE creators (
    id text PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    name text NOT NULL,
    bio text NOT NULL DEFAULT '',
    avatar_media_id text REFERENCES catalog_media(id),
    links jsonb NOT NULL DEFAULT '[]',
    user_id text UNIQUE REFERENCES users(id),
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE artifact_creators (
    artifact_id text NOT NULL REFERENCES catalog_artifacts(id) ON DELETE CASCADE,
    creator_id text NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
    position integer NOT NULL DEFAULT 0,
    PRIMARY KEY (artifact_id, creator_id)
);
CREATE INDEX artifact_creators_creator ON artifact_creators(creator_id);

CREATE TABLE posts (
    id text PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    title text NOT NULL,
    summary text NOT NULL DEFAULT '',
    body text NOT NULL DEFAULT '',
    cover_media_id text REFERENCES catalog_media(id),
    author_id text REFERENCES creators(id) ON DELETE SET NULL,
    tags text[] NOT NULL DEFAULT '{}',
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'published')),
    published_at timestamptz,
    created_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX posts_published ON posts(published_at DESC) WHERE state = 'published';

-- '' means the highest-ranked plan.
ALTER TABLE access_grants ADD COLUMN plan text NOT NULL DEFAULT '';

ALTER TABLE instances DROP CONSTRAINT instances_kind_check;
ALTER TABLE instances ADD CONSTRAINT instances_kind_check
    CHECK (kind IN ('workspace', 'preview', 'demo'));
ALTER TABLE instances ADD COLUMN expires_at timestamptz;
CREATE INDEX instances_demo_expiry ON instances(expires_at) WHERE kind = 'demo';

CREATE TABLE api_tokens (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id),
    name text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at timestamptz,
    revoked_at timestamptz
);
