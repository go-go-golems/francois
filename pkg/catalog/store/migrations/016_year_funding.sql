-- Confirmed one-time funding reuses the existing durable checkout record.
ALTER TABLE checkout_attempts ADD COLUMN support_paid_at timestamptz;
ALTER TABLE checkout_attempts ADD COLUMN support_until timestamptz;
