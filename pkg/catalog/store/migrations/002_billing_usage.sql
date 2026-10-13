-- Stripe Customer mapping and per-subscription projections.
CREATE TABLE billing_customers (
    user_id text PRIMARY KEY REFERENCES users(id),
    stripe_customer_id text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE billing_subscriptions (
    stripe_subscription_id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id),
    stripe_customer_id text NOT NULL,
    price_id text NOT NULL DEFAULT '',
    status text NOT NULL,
    quantity integer NOT NULL DEFAULT 0,
    item_count integer NOT NULL DEFAULT 0,
    access_until timestamptz,
    trial_until timestamptz,
    cancel_at_period_end boolean NOT NULL DEFAULT false,
    canceled_at timestamptz,
    livemode boolean NOT NULL DEFAULT false,
    refreshed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX billing_subscriptions_user ON billing_subscriptions (user_id);

CREATE TABLE checkout_attempts (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id),
    plan text NOT NULL,
    state text NOT NULL CHECK (state IN ('pending', 'open', 'completed', 'expired', 'failed')),
    session_id text UNIQUE,
    session_url text,
    expires_at timestamptz,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- At most one admitted attempt per user; terminal states release the slot.
CREATE UNIQUE INDEX checkout_attempts_open ON checkout_attempts (user_id) WHERE state IN ('pending', 'open');

CREATE TABLE stripe_events (
    event_id text PRIMARY KEY,
    type text NOT NULL,
    customer_id text NOT NULL DEFAULT '',
    subscription_id text NOT NULL DEFAULT '',
    session_id text NOT NULL DEFAULT '',
    livemode boolean NOT NULL DEFAULT false,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'processed', 'ignored', 'error')),
    attempts integer NOT NULL DEFAULT 0,
    lease_token text,
    lease_until timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    error_code text,
    stripe_created_at timestamptz,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz
);
CREATE INDEX stripe_events_due ON stripe_events (next_attempt_at) WHERE state = 'pending';

-- Included model allowance, reservations and per-call settlement.
CREATE TABLE model_allowance_periods (
    user_id text NOT NULL REFERENCES users(id),
    period text NOT NULL,
    included integer NOT NULL,
    adjustments integer NOT NULL DEFAULT 0,
    reserved integer NOT NULL DEFAULT 0,
    used integer NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, period),
    CHECK (reserved >= 0 AND used >= 0)
);

CREATE TABLE model_calls (
    id text PRIMARY KEY,
    call_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id),
    instance_id text NOT NULL,
    period text NOT NULL,
    settings_revision bigint NOT NULL,
    model text NOT NULL,
    credits integer NOT NULL,
    state text NOT NULL CHECK (state IN ('reserved', 'started', 'completed', 'failed', 'released', 'review')),
    input_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    cache_read_tokens bigint NOT NULL DEFAULT 0,
    cache_write_tokens bigint NOT NULL DEFAULT 0,
    cost_total double precision NOT NULL DEFAULT 0,
    rounds integer NOT NULL DEFAULT 0,
    model_used text NOT NULL DEFAULT '',
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    settled_at timestamptz
);
CREATE INDEX model_calls_user_time ON model_calls (user_id, created_at DESC);
CREATE INDEX model_calls_open ON model_calls (state) WHERE state IN ('reserved', 'started');

CREATE TABLE allowance_adjustments (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id),
    period text NOT NULL,
    credits integer NOT NULL,
    reason text NOT NULL,
    actor_id text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
