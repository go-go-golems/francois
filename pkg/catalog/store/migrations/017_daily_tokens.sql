-- Credits remain historical measurements; new calls use generated tokens.
ALTER TABLE model_calls ADD COLUMN reserved_tokens integer NOT NULL DEFAULT 0;
ALTER TABLE model_calls ADD COLUMN charged_tokens integer NOT NULL DEFAULT 0;
CREATE TABLE daily_token_usage (
 user_id text NOT NULL REFERENCES users(id), period text NOT NULL,
 used integer NOT NULL DEFAULT 0 CHECK(used>=0),
 reserved integer NOT NULL DEFAULT 0 CHECK(reserved>=0),
 PRIMARY KEY(user_id,period)
);
CREATE TABLE token_ceiling_overrides (
 user_id text PRIMARY KEY REFERENCES users(id),
 daily_tokens integer NOT NULL CHECK(daily_tokens>=0 AND daily_tokens<=100000000)
);
-- Old in-flight credit reservations cannot be resumed by a new process.
UPDATE model_calls SET state=CASE WHEN state='started' THEN 'review' ELSE 'released' END,
 error_code='token_policy_migration',settled_at=now() WHERE state IN('reserved','started');
-- Only the latest settings revision is active. Preserve historical JSON.
UPDATE platform_settings SET document=jsonb_set(document,'{plans}',(
 SELECT jsonb_agg((p-'monthlyCredits'-'dailyCredits')||jsonb_build_object('dailyTokens',CASE WHEN p->>'key'='free' THEN 10000 ELSE 100000 END))
 FROM jsonb_array_elements(document->'plans')p
)) WHERE revision=(SELECT max(revision)FROM platform_settings);
UPDATE platform_settings SET document=jsonb_set(document,'{allowance,maxOutputTokens}','8192') WHERE revision=(SELECT max(revision)FROM platform_settings);

UPDATE platform_settings SET document=jsonb_set(document,'{models,allowed}',COALESCE((SELECT jsonb_agg(m-'credits')FROM jsonb_array_elements(document#>'{models,allowed}')m),'[]')) WHERE revision=(SELECT max(revision)FROM platform_settings);
UPDATE platform_settings SET document=jsonb_set(document,'{demo}',((document->'demo')-'credits')||jsonb_build_object('tokens',5000,'dailyBudget',200000)) WHERE revision=(SELECT max(revision)FROM platform_settings);
