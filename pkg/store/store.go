// Package store persists artifacts and their application state in SQLite.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }
type Artifact struct {
	ID        string                     `json:"id"`
	Title     string                     `json:"title"`
	Version   int64                      `json:"version"`
	Manifest  map[string]json.RawMessage `json:"manifest"`
	Port      int                        `json:"port"`
	UpdatedAt string                     `json:"updatedAt"`
}

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.Wrap(err, "create data directory")
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "francois.db"))
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	_, err = db.ExecContext(ctx, `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS artifacts(id TEXT PRIMARY KEY,title TEXT NOT NULL,version INTEGER NOT NULL,manifest TEXT NOT NULL,port INTEGER NOT NULL DEFAULT 0,updated_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS versions(artifact TEXT NOT NULL,version INTEGER NOT NULL,html BLOB NOT NULL,PRIMARY KEY(artifact,version));
 CREATE TABLE IF NOT EXISTS docs(artifact TEXT NOT NULL,path TEXT NOT NULL,body TEXT NOT NULL,version INTEGER NOT NULL,PRIMARY KEY(artifact,path));
 CREATE TABLE IF NOT EXISTS leases(artifact TEXT NOT NULL,path TEXT NOT NULL,holder TEXT NOT NULL,expires INTEGER NOT NULL,data TEXT,PRIMARY KEY(artifact,path));
 CREATE TABLE IF NOT EXISTS assets(id TEXT PRIMARY KEY,artifact TEXT NOT NULL,type TEXT NOT NULL,bytes BLOB NOT NULL,created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS threads(id TEXT PRIMARY KEY,artifact TEXT NOT NULL,anchor TEXT NOT NULL,resolved INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS comments(id TEXT PRIMARY KEY,thread TEXT NOT NULL,author TEXT NOT NULL,text TEXT NOT NULL,created_at TEXT NOT NULL);`)
	if err != nil {
		_ = db.Close()
		return nil, errors.Wrap(err, "initialize database")
	}
	return s, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Import(ctx context.Context, id, title string, html []byte, manifest map[string]json.RawMessage) (Artifact, error) {
	if !validID.MatchString(id) {
		return Artifact{}, errors.New("id must contain lowercase letters, digits, underscores or hyphens")
	}
	if len(html) == 0 || len(html) > 16<<20 {
		return Artifact{}, errors.New("HTML must be between 1 byte and 16 MiB")
	}
	m, err := json.Marshal(manifest)
	if err != nil {
		return Artifact{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = tx.ExecContext(ctx, `INSERT INTO artifacts(id,title,version,manifest,updated_at) VALUES(?,?,1,?,?)`, id, title, string(m), now); err != nil {
		return Artifact{}, errors.Wrap(err, "register artifact (choose a unique id)")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO versions VALUES(?,1,?)`, id, html); err != nil {
		return Artifact{}, err
	}
	if err = tx.Commit(); err != nil {
		return Artifact{}, err
	}
	return s.Get(ctx, id)
}
func scanArtifact(scan func(...any) error) (Artifact, error) {
	var a Artifact
	var m string
	if err := scan(&a.ID, &a.Title, &a.Version, &m, &a.Port, &a.UpdatedAt); err != nil {
		return a, err
	}
	if err := json.Unmarshal([]byte(m), &a.Manifest); err != nil {
		return a, err
	}
	return a, nil
}
func (s *Store) Get(ctx context.Context, id string) (Artifact, error) {
	return scanArtifact(s.DB.QueryRowContext(ctx, `SELECT id,title,version,manifest,port,updated_at FROM artifacts WHERE id=?`, id).Scan)
}
func (s *Store) List(ctx context.Context) ([]Artifact, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,title,version,manifest,port,updated_at FROM artifacts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Artifact{}
	for rows.Next() {
		a, err := scanArtifact(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) HTML(ctx context.Context, id string, version int64) ([]byte, error) {
	var b []byte
	err := s.DB.QueryRowContext(ctx, `SELECT html FROM versions WHERE artifact=? AND version=?`, id, version).Scan(&b)
	return b, err
}
func (s *Store) SetPort(ctx context.Context, id string, port int) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE artifacts SET port=? WHERE id=?`, port, id)
	return err
}
func (s *Store) Publish(ctx context.Context, id string, expected int64, html []byte) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := tx.ExecContext(ctx, `UPDATE artifacts SET version=version+1,updated_at=? WHERE id=? AND version=?`, time.Now().UTC().Format(time.RFC3339), id, expected)
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, errors.New("conflict")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO versions VALUES(?,?,?)`, id, expected+1, html); err != nil {
		return 0, err
	}
	return expected + 1, tx.Commit()
}
func (s *Store) Identity(ctx context.Context, newID string) (string, error) {
	if _, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO settings VALUES('owner',?)`, newID); err != nil {
		return "", err
	}
	var id string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='owner'`).Scan(&id)
	return id, err
}
