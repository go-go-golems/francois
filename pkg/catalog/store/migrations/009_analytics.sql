-- First-party portal page views. No IPs, full user agents or query strings.
CREATE TABLE traffic_views (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    site text NOT NULL,
    path text NOT NULL,
    visitor text NOT NULL,
    session text NOT NULL,
    user_id text NOT NULL DEFAULT '',
    referrer text NOT NULL DEFAULT '',
    source text NOT NULL DEFAULT '',
    medium text NOT NULL DEFAULT '',
    campaign text NOT NULL DEFAULT '',
    device text NOT NULL,
    browser text NOT NULL
);
CREATE INDEX traffic_views_time ON traffic_views(created_at);
CREATE INDEX model_calls_time ON model_calls(created_at);
