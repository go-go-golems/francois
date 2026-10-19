-- Checkout completion and payment settlement are independent facts.
ALTER TABLE checkout_attempts ADD COLUMN payment_state text
 CHECK (payment_state IN ('pending','paid','failed','refunded'));
-- Unconfirmed old completions require authoritative reconciliation, including
-- old refunded rows whose cleared dates cannot tell us their payment outcome.
UPDATE checkout_attempts SET payment_state=CASE
 WHEN support_until IS NOT NULL THEN 'paid'
 WHEN state IN ('expired','failed') THEN 'failed'
 ELSE 'pending' END
WHERE price='support:year:99999999';
CREATE INDEX checkout_year_payments_due ON checkout_attempts(updated_at)
 WHERE payment_state='pending' AND session_id IS NOT NULL;
