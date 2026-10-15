-- A Checkout attempt is for one Stripe price; a request for another price
-- supersedes it instead of reusing its Session. Rows from before plans keep
-- their old value ('subscriber'), which matches no price.
ALTER TABLE checkout_attempts RENAME COLUMN plan TO price;
