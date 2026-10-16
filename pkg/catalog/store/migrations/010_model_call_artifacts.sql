-- Snapshot artifact identity so historical usage survives instance deletion.
-- Missing historical relationships remain explicitly unknown (empty ID).
ALTER TABLE model_calls ADD COLUMN artifact_id text NOT NULL DEFAULT '';
UPDATE model_calls c SET artifact_id=r.artifact_id
FROM instances i JOIN releases r ON r.id=i.release_id
WHERE i.id=c.instance_id;
