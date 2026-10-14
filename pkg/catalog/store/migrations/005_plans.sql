-- FRANCOIS-004: plans replace the global per-user limits and credits.
-- Settings documents are parsed strictly, so every stored revision is
-- rewritten into the new shape: the old instance, member and credit limits
-- become a "subscriber" plan whose prices are the prices existing
-- subscriptions pay (so current subscribers keep their access), next to a
-- free plan. Demo stays disabled; the storefront wording starts empty.

WITH subscriber_prices AS (
    SELECT COALESCE(jsonb_agg(jsonb_build_object('interval', 'month', 'priceId', price_id, 'label', '') ORDER BY price_id), '[]'::jsonb) AS prices
    FROM (SELECT DISTINCT price_id FROM billing_subscriptions WHERE price_id LIKE 'price\_%') p
)
UPDATE platform_settings SET document =
    (document
        #- '{allowance,monthlyCredits}' #- '{allowance,dailyCredits}'
        #- '{limits,instancesPerUser}' #- '{limits,membersPerInstance}')
    || jsonb_build_object(
        'limits', (document->'limits' #- '{instancesPerUser}' #- '{membersPerInstance}') || jsonb_build_object('maxMediaBytes', 26214400),
        'plans', jsonb_build_array(
            jsonb_build_object('key', 'free', 'name', 'Free', 'rank', 0, 'description', '', 'highlights', '[]'::jsonb, 'public', true,
                'prices', '[]'::jsonb, 'instances', 0, 'membersPerInstance', 1, 'monthlyCredits', 0, 'dailyCredits', 0, 'models', '[]'::jsonb),
            jsonb_build_object('key', 'subscriber', 'name', 'Subscriber', 'rank', 1, 'description', '', 'highlights', '[]'::jsonb, 'public', true,
                'prices', (SELECT prices FROM subscriber_prices),
                'instances', COALESCE(document->'limits'->'instancesPerUser', '10'::jsonb),
                'membersPerInstance', COALESCE(document->'limits'->'membersPerInstance', '10'::jsonb),
                'monthlyCredits', COALESCE(document->'allowance'->'monthlyCredits', '0'::jsonb),
                'dailyCredits', COALESCE(document->'allowance'->'dailyCredits', '0'::jsonb),
                'models', '[]'::jsonb)),
        'demo', jsonb_build_object('enabled', false, 'ttlMinutes', 30, 'maxActive', 20, 'perAddressPerHour', 3, 'credits', 5, 'dailyBudget', 200, 'models', '[]'::jsonb),
        'storefront', jsonb_build_object('title', '', 'tagline', '', 'intro', ''))
WHERE NOT document ? 'plans';
