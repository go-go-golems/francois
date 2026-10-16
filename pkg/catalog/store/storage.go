package store

import (
	"context"
	"encoding/json"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

// StoragePaused carries a safe notice for a rejected persistent mutation.
type StoragePaused struct{ Mode, Message string }

func (e *StoragePaused) Error() string { return e.Message }

// beginStorageWrite serializes every persistent mutation of an instance.
// Checking policy here also covers direct store callers and changed settings.
func (s *Store) beginStorageWrite(ctx context.Context, instance string) (pgx.Tx, settings.Limits, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, settings.Limits{}, err
	}
	fail := func(err error) (pgx.Tx, settings.Limits, error) {
		_ = tx.Rollback(ctx)
		return nil, settings.Limits{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('storage:'||$1))`, instance); err != nil {
		return fail(err)
	}
	d := settings.Defaults()
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT document FROM platform_settings ORDER BY revision DESC LIMIT 1`).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fail(err)
	}
	if err == nil {
		if err = json.Unmarshal(raw, &d); err != nil {
			return fail(err)
		}
	}
	switch d.Operations.StorageMode {
	case "read-only":
		return fail(&StoragePaused{"read-only", d.Operations.Message("Saving is temporarily paused. Your existing work is still available.")})
	case "off":
		return fail(&StoragePaused{"off", d.Operations.Message("Saved work is temporarily unavailable.")})
	}
	return tx, d.Limits, nil
}

// storageBytes counts logical payload bytes. Database/index overhead is separate.
func storageBytes(ctx context.Context, tx pgx.Tx, instance string) (int64, error) {
	var n int64
	err := tx.QueryRow(ctx, `SELECT
 (SELECT COALESCE(sum(size),0) FROM instance_documents WHERE instance_id=$1)+
 (SELECT COALESCE(sum(octet_length(bytes)),0) FROM instance_assets WHERE instance_id=$1)+
 (SELECT COALESCE(sum(octet_length(anchor::text)),0) FROM instance_threads WHERE instance_id=$1)+
 (SELECT COALESCE(sum(octet_length(c.text)),0) FROM instance_comments c JOIN instance_threads t ON t.id=c.thread_id WHERE t.instance_id=$1)+
 (SELECT COALESCE(sum(octet_length(data::text)),0) FROM instance_leases WHERE instance_id=$1)`, instance).Scan(&n)
	return n, err
}

// commitStorageWrite permits cleanup/shrinking even above a lowered quota.
func commitStorageWrite(ctx context.Context, tx pgx.Tx, instance string, before, limit int64) error {
	after, err := storageBytes(ctx, tx, instance)
	if err != nil {
		return err
	}
	if after > limit && after > before {
		return errors.Wrap(ErrLimit, "Saved work has reached its storage limit")
	}
	return tx.Commit(ctx)
}
