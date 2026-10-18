-- Monthly support is a contribution, independent of access-plan entitlements.
ALTER TABLE billing_subscriptions ADD COLUMN support_amount_cents bigint NOT NULL DEFAULT 0;
ALTER TABLE billing_subscriptions ADD COLUMN started_at timestamptz;
