package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	local "github.com/go-go-golems/francois/pkg/store"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

// PrivateOwner returns the user ID that owns a private data/users/<uid> path,
// or "" for shared paths.
func PrivateOwner(path string) string {
	if !strings.HasPrefix(path, "data/users/") {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

func collectionOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return ""
	}
	return path[:i]
}

// Document reads one document within an instance; private paths of another
// user read as missing.
func (s *Store) Document(ctx context.Context, instance, user, path string) (local.Document, error) {
	d := local.Document{ID: path[strings.LastIndex(path, "/")+1:], Path: path}
	if owner := PrivateOwner(path); owner != "" && owner != user {
		return d, nil
	}
	var body []byte
	err := s.Pool.QueryRow(ctx, `SELECT body,version FROM instance_documents WHERE instance_id=$1 AND path=$2`, instance, path).Scan(&body, &d.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	d.Exists = true
	return d, json.Unmarshal(body, &d.Body)
}

// QueryDocuments scopes rows by instance, collection and private owner in SQL
// before applying the shared filter/order/limit semantics.
func (s *Store) QueryDocuments(ctx context.Context, instance, user string, q local.Query) ([]local.Document, error) {
	if err := local.ValidateQuery(q); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT path,body,version FROM instance_documents
WHERE instance_id=$1 AND collection=$2 AND (path NOT LIKE 'data/users/%' OR split_part(path,'/',3)=$3)
ORDER BY path LIMIT 5001`, instance, q.Path, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs := []local.Document{}
	for rows.Next() {
		var d local.Document
		var body []byte
		if err = rows.Scan(&d.Path, &body, &d.Version); err != nil {
			return nil, err
		}
		if len(docs) >= 5000 {
			return nil, errors.New("Query exceeds 5000 document scan limit")
		}
		if err = json.Unmarshal(body, &d.Body); err != nil {
			return nil, err
		}
		d.ID = d.Path[len(q.Path)+1:]
		d.Exists = true
		docs = append(docs, d)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return local.Apply(docs, q), nil
}

// WriteDocument applies set/update/delete within an instance, enforcing the
// private namespace and the per-instance document limit.
func (s *Store) WriteDocument(ctx context.Context, instance, user, path, op string, body map[string]any, maxDocs int) error {
	if owner := PrivateOwner(path); owner != "" && owner != user {
		return errors.New("Private document belongs to another person")
	}
	tx, limits, err := s.beginStorageWrite(ctx, instance)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	before, err := storageBytes(ctx, tx, instance)
	if err != nil {
		return err
	}
	maxDocs = min(maxDocs, limits.DocumentsPerInstance)
	if op == "delete" {
		if _, err = tx.Exec(ctx, `DELETE FROM instance_documents WHERE instance_id=$1 AND path=$2`, instance, path); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if op == "update" {
		var old []byte
		if err = tx.QueryRow(ctx, `SELECT body FROM instance_documents WHERE instance_id=$1 AND path=$2 FOR UPDATE`, instance, path).Scan(&old); err != nil {
			return errors.New("Cannot update a missing document")
		}
		var data map[string]any
		if err = json.Unmarshal(old, &data); err != nil {
			return err
		}
		body = local.Merge(data, body)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if len(raw) > 262144 {
		return errors.New("Document exceeds 256 KiB")
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM instance_documents WHERE instance_id=$1 AND path=$2)`, instance, path).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		var n int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM instance_documents WHERE instance_id=$1`, instance).Scan(&n); err != nil {
			return err
		}
		if n >= maxDocs {
			return errors.New("Instance document limit reached")
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO instance_documents(instance_id,path,collection,body,size,updated_by) VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(instance_id,path) DO UPDATE SET body=EXCLUDED.body,size=EXCLUDED.size,version=instance_documents.version+1,updated_by=EXCLUDED.updated_by,updated_at=now()`,
		instance, path, collectionOf(path), raw, len(raw), user); err != nil {
		return err
	}
	return commitStorageWrite(ctx, tx, instance, before, limits.StorageBytesPerInstance)
}

// AcquireLease takes or renews a cooperative lease within an instance.
func (s *Store) AcquireLease(ctx context.Context, instance, user, path, holder string, ttl int64, data any) (bool, error) {
	if owner := PrivateOwner(path); owner != "" && owner != user {
		return false, errors.New("Private document belongs to another person")
	}
	if holder == "" || len(holder) > 200 || ttl < 1000 || ttl > 600000 {
		return false, errors.New("Invalid lease holder or TTL")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	tx, limits, err := s.beginStorageWrite(ctx, instance)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	before, err := storageBytes(ctx, tx, instance)
	if err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO instance_leases(instance_id,path,holder,expires_at,data) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(instance_id,path) DO UPDATE SET holder=EXCLUDED.holder,expires_at=EXCLUDED.expires_at,data=EXCLUDED.data
WHERE instance_leases.expires_at<=now() OR instance_leases.holder=EXCLUDED.holder`, instance, path, holder, time.Now().Add(time.Duration(ttl)*time.Millisecond), raw)
	if err != nil {
		return false, err
	}
	if err = commitStorageWrite(ctx, tx, instance, before, limits.StorageBytesPerInstance); err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
