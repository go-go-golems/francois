-- Bring stored settings into the current strict schema, as in migration 005.
-- Preserve accounts, plans, content and model accounting. Lower caps keep data readable.
UPDATE platform_settings SET document = document || jsonb_build_object(
 'operations', jsonb_build_object('acceptNewVisits',true,'samplingEnabled',true,'storageMode','read-write','notice',''),
 'allowance', (document->'allowance') || jsonb_build_object('perUserConcurrency',1,'globalConcurrency',LEAST((document->'allowance'->>'globalConcurrency')::int,6)),
 'limits', (document->'limits') || jsonb_build_object('viewsPerUser',3,'storageBytesPerInstance',16777216,'documentsPerInstance',LEAST((document->'limits'->>'documentsPerInstance')::int,500)),
 'demo', (document->'demo') || jsonb_build_object('perUserPerHour',3,'ttlMinutes',LEAST((document->'demo'->>'ttlMinutes')::int,20))
);
