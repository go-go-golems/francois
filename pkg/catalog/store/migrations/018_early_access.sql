-- A concrete date replaces permanent artifact-plan restrictions.
ALTER TABLE catalog_artifacts ADD COLUMN early_access_until timestamptz;
ALTER TABLE catalog_artifacts DROP COLUMN required_plan;
-- Existing published artifacts start open to Free accounts. New artifacts
-- use the configured period only on their first publication.
UPDATE platform_settings SET document=jsonb_set(document,'{publishing,earlyAccessDays}','0')
WHERE revision=(SELECT max(revision)FROM platform_settings);
-- Exactly two resource classes. Preserve actual billing price acceptance,
-- not arbitrary plan ranks or historic credit-to-token conversion factors.
UPDATE platform_settings SET document=jsonb_set(document,'{plans}',jsonb_build_array(
 jsonb_build_object('key','free','name','Free','prices','[]'::jsonb,
  'instances',GREATEST(COALESCE((SELECT (p->>'instances')::int FROM jsonb_array_elements(document->'plans')p WHERE p->>'key'='free'),1),1),
  'membersPerInstance',COALESCE((SELECT (p->>'membersPerInstance')::int FROM jsonb_array_elements(document->'plans')p WHERE p->>'key'='free'),1),
  'dailyTokens',10000),
 jsonb_build_object('key','subscriber','name','Paying',
  'prices',COALESCE((SELECT jsonb_agg(pr)FROM jsonb_array_elements(document->'plans')p CROSS JOIN LATERAL jsonb_array_elements(p->'prices')pr),'[]'::jsonb),
  'instances',COALESCE((SELECT (p->>'instances')::int FROM jsonb_array_elements(document->'plans')p WHERE p->>'key'='subscriber'),10),
  'membersPerInstance',COALESCE((SELECT (p->>'membersPerInstance')::int FROM jsonb_array_elements(document->'plans')p WHERE p->>'key'='subscriber'),5),
  'dailyTokens',100000)))
WHERE revision=(SELECT max(revision)FROM platform_settings);

UPDATE platform_settings SET document=jsonb_set(document,'{demo}',(document->'demo')-'models') WHERE revision=(SELECT max(revision)FROM platform_settings);
