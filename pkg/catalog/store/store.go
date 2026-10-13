// Package store persists the hosted catalog's accounts, releases, instances,
// billing projections and usage records in PostgreSQL.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationLock int64 = 4_202_610

// ErrNotFound is returned when a scoped lookup has no visible row.
var ErrNotFound = errors.New("not found")

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.Wrap(err, "parse database URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "connect database")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.Wrap(err, "ping database")
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.Pool.Ping(ctx) }

// Migrate applies embedded migrations in order under an advisory lock and
// refuses to run when an applied migration's contents changed.
func (s *Store) Migrate(ctx context.Context) ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, errors.Wrap(err, "read migrations")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLock); err != nil {
		return nil, errors.Wrap(err, "lock migrations")
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY, checksum bytea NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return nil, errors.Wrap(err, "create migration ledger")
	}
	applied := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(body)
		var existing []byte
		err = tx.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name=$1`, e.Name()).Scan(&existing)
		if err == nil {
			if string(existing) != string(sum[:]) {
				return nil, errors.Errorf("migration %s changed after it was applied", e.Name())
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			return nil, errors.Wrapf(err, "apply migration %s", e.Name())
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(name,checksum) VALUES($1,$2)`, e.Name(), sum[:]); err != nil {
			return nil, err
		}
		applied = append(applied, e.Name())
	}
	return applied, tx.Commit(ctx)
}

// Pending reports migrations that have not been applied, so serve can refuse
// to start against an incompatible schema.
func (s *Store) Pending(ctx context.Context) ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	var exists bool
	if err = s.Pool.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return nil, err
	}
	done := map[string]bool{}
	if exists {
		rows, err := s.Pool.Query(ctx, `SELECT name FROM schema_migrations`)
		if err != nil {
			return nil, err
		}
		names, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			done[n] = true
		}
	}
	pending := []string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") && !done[e.Name()] {
			pending = append(pending, e.Name())
		}
	}
	return pending, nil
}

// NewID returns an opaque prefixed identifier.
func NewID(prefix string) string {
	return prefix + RandomHex(12)
}

func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Hash is the storage form of bearer secrets (sessions, codes, grants).
func Hash(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
