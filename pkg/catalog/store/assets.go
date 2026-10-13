package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

type Asset struct {
	ID          string    `json:"id"`
	MediaType   string    `json:"contentType"`
	Size        int64     `json:"sizeBytes"`
	OwnerUserID string    `json:"ownerUserId"`
	CreatedAt   time.Time `json:"createdAt"`
}

// PutAsset stores bytes for an instance within its byte and count limits.
func (s *Store) PutAsset(ctx context.Context, instance, user, kind string, data []byte, maxBytes int64) (Asset, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Asset{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('assets:'||$1))`, instance); err != nil {
		return Asset{}, err
	}
	var count, total int64
	if err = tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(length(bytes)),0) FROM instance_assets WHERE instance_id=$1`, instance).Scan(&count, &total); err != nil {
		return Asset{}, err
	}
	if count >= 1000 || total+int64(len(data)) > maxBytes {
		return Asset{}, errors.Wrap(ErrLimit, "Asset quota reached")
	}
	a := Asset{ID: NewID("b_"), MediaType: kind, Size: int64(len(data)), OwnerUserID: user}
	if err = tx.QueryRow(ctx, `INSERT INTO instance_assets(id,instance_id,owner_user_id,media_type,bytes) VALUES($1,$2,$3,$4,$5) RETURNING created_at`, a.ID, instance, user, kind, data).Scan(&a.CreatedAt); err != nil {
		return Asset{}, err
	}
	return a, tx.Commit(ctx)
}

func (s *Store) AssetBytes(ctx context.Context, instance, id string) (string, []byte, error) {
	var kind string
	var data []byte
	err := s.Pool.QueryRow(ctx, `SELECT media_type,bytes FROM instance_assets WHERE instance_id=$1 AND id=$2`, instance, id).Scan(&kind, &data)
	return kind, data, notFound(err)
}

func (s *Store) Assets(ctx context.Context, instance string) ([]Asset, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,media_type,length(bytes),owner_user_id,created_at FROM instance_assets WHERE instance_id=$1 ORDER BY created_at`, instance)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Asset, error) {
		var a Asset
		err := r.Scan(&a.ID, &a.MediaType, &a.Size, &a.OwnerUserID, &a.CreatedAt)
		return a, err
	})
}

// DeleteAsset lets the uploader or an instance owner delete an asset.
func (s *Store) DeleteAsset(ctx context.Context, instance, id, user string, owner bool) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM instance_assets WHERE instance_id=$1 AND id=$2 AND ($3 OR owner_user_id=$4)`, instance, id, owner, user)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// Comments.

type Comment struct {
	ThreadID  string          `json:"threadId"`
	ID        string          `json:"id"`
	Anchor    json.RawMessage `json:"anchor"`
	Resolved  bool            `json:"resolved"`
	AuthorID  string          `json:"authorId"`
	Author    string          `json:"author"`
	Text      string          `json:"text"`
	CreatedAt time.Time       `json:"createdAt"`
}

func (s *Store) CreateThread(ctx context.Context, instance, user string, anchor []byte, text string) (string, string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	thread, comment := NewID("t_"), NewID("m_")
	if _, err = tx.Exec(ctx, `INSERT INTO instance_threads(id,instance_id,anchor,created_by) VALUES($1,$2,$3,$4)`, thread, instance, anchor, user); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO instance_comments(id,thread_id,author_id,text) VALUES($1,$2,$3,$4)`, comment, thread, user, text); err != nil {
		return "", "", err
	}
	return thread, comment, tx.Commit(ctx)
}

func (s *Store) threadInInstance(ctx context.Context, instance, thread string) error {
	var ok bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM instance_threads WHERE id=$1 AND instance_id=$2)`, thread, instance).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("Thread not found")
	}
	return nil
}

func (s *Store) Reply(ctx context.Context, instance, thread, user, text string) error {
	if err := s.threadInInstance(ctx, instance, thread); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO instance_comments(id,thread_id,author_id,text) VALUES($1,$2,$3,$4)`, NewID("m_"), thread, user, text)
	return err
}

func (s *Store) ResolveThread(ctx context.Context, instance, thread string, resolved bool) error {
	if err := s.threadInInstance(ctx, instance, thread); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `UPDATE instance_threads SET resolved=$2 WHERE id=$1`, thread, resolved)
	return err
}

// DeleteThread allows the thread creator or an instance owner.
func (s *Store) DeleteThread(ctx context.Context, instance, thread, user string, owner bool) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM instance_threads WHERE id=$1 AND instance_id=$2 AND ($3 OR created_by=$4)`, thread, instance, owner, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("Thread not found or not yours")
	}
	return nil
}
