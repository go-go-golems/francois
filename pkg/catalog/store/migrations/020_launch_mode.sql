-- How the storefront starts an artifact: inline in the page (and from the
-- archive's Open), or full screen in a new tab after showing the cover.
ALTER TABLE catalog_artifacts ADD COLUMN launch_mode text NOT NULL DEFAULT 'tab'
  CHECK (launch_mode IN ('inline','tab'));
