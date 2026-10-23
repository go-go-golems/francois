-- Monthly support bought as a company ("enterprise"): a power-of-two amount
-- invoiced with the company's name, address and tax ID. Empty for individuals.
ALTER TABLE billing_subscriptions ADD COLUMN support_tier text NOT NULL DEFAULT ''
  CHECK (support_tier IN ('','enterprise'));
