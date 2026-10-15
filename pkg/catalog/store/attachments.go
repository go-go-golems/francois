package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Attachment is a supplementary file on a release: a manual, an archive,
// notes. Attachments do not run, so unlike release files they can be added,
// relabelled and removed after publishing.
type Attachment struct {
	ID         string    `json:"id"`
	ReleaseID  string    `json:"releaseId"`
	Name       string    `json:"name"`
	Label      string    `json:"label"`
	MediaType  string    `json:"mediaType"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"`
	StorageKey string    `json:"-"`
	Position   int       `json:"position"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

const attachmentColumns = `id,release_id,name,label,media_type,size,sha256,storage_key,position,created_by,created_at`

func scanAttachment(r pgx.Row) (Attachment, error) {
	var a Attachment
	err := r.Scan(&a.ID, &a.ReleaseID, &a.Name, &a.Label, &a.MediaType, &a.Size, &a.SHA256, &a.StorageKey, &a.Position, &a.CreatedBy, &a.CreatedAt)
	return a, notFound(err)
}

// CreateAttachment appends an attachment to a release unless it already
// holds limit attachments (ErrConflict). The release row is locked so
// concurrent uploads cannot exceed the limit.
func (s *Store) CreateAttachment(ctx context.Context, a Attachment, limit int) (Attachment, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Attachment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var n, next int
	if err = tx.QueryRow(ctx, `SELECT 1 FROM releases WHERE id=$1 FOR UPDATE`, a.ReleaseID).Scan(&n); err != nil {
		return Attachment{}, notFound(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*), COALESCE(max(position)+1,0) FROM release_attachments WHERE release_id=$1`, a.ReleaseID).Scan(&n, &next); err != nil {
		return Attachment{}, err
	}
	if n >= limit {
		return Attachment{}, ErrConflict
	}
	out, err := scanAttachment(tx.QueryRow(ctx, `INSERT INTO release_attachments(id,release_id,name,label,media_type,size,sha256,storage_key,position,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+attachmentColumns,
		NewID("att_"), a.ReleaseID, a.Name, a.Label, a.MediaType, a.Size, a.SHA256, a.StorageKey, next, a.CreatedBy))
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Store) Attachment(ctx context.Context, id string) (Attachment, error) {
	return scanAttachment(s.Pool.QueryRow(ctx, `SELECT `+attachmentColumns+` FROM release_attachments WHERE id=$1`, id))
}

// UpdateAttachment changes the label and/or position; nil leaves a field.
func (s *Store) UpdateAttachment(ctx context.Context, id string, label *string, position *int) (Attachment, error) {
	return scanAttachment(s.Pool.QueryRow(ctx, `UPDATE release_attachments SET label=COALESCE($2,label), position=COALESCE($3,position) WHERE id=$1 RETURNING `+attachmentColumns,
		id, label, position))
}

// DeleteAttachment removes the row; the blob stays (blobs are shared by
// content hash).
func (s *Store) DeleteAttachment(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM release_attachments WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// attachments loads the attachments of releases, in display order.
func (s *Store) attachments(ctx context.Context, releases []Release, index map[string]int) error {
	ids := make([]string, len(releases))
	for i, r := range releases {
		ids[i] = r.ID
		releases[i].Attachments = []Attachment{}
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+attachmentColumns+` FROM release_attachments WHERE release_id=ANY($1) ORDER BY position, created_at`, ids)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Attachment, error) { return scanAttachment(r) })
	if err != nil {
		return err
	}
	for _, a := range list {
		i := index[a.ReleaseID]
		releases[i].Attachments = append(releases[i].Attachments, a)
	}
	return nil
}
