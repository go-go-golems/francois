-- FRANCOIS-005: video posters and release attachments.

-- A video's still frame: an image in the media library.
ALTER TABLE catalog_media ADD COLUMN poster_media_id text REFERENCES catalog_media(id) ON DELETE SET NULL;

-- Supplementary files on a release (manuals, archives, notes). They do not
-- run, so unlike release_files they can change after publishing.
CREATE TABLE release_attachments (
    id text PRIMARY KEY,
    release_id text NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    name text NOT NULL,
    label text NOT NULL DEFAULT '',
    media_type text NOT NULL,
    size bigint NOT NULL,
    sha256 text NOT NULL,
    storage_key text NOT NULL,
    position integer NOT NULL DEFAULT 0,
    created_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX release_attachments_release ON release_attachments(release_id, position, created_at);

-- Settings are parsed strictly: every stored revision gains the new limit.
UPDATE platform_settings SET document = jsonb_set(document, '{limits,maxAttachmentBytes}', '104857600')
WHERE NOT document->'limits' ? 'maxAttachmentBytes';
