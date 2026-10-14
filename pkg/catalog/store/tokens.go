package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/jackc/pgx/v5"
)

// TokenPrefix marks personal API tokens.
const TokenPrefix = "frt_"

// APIToken is a personal token for the studio API. Only its hash is stored;
// the secret is shown once at creation.
type APIToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

const tokenColumns = `id,user_id,name,created_at,last_used_at,expires_at,revoked_at`

func scanToken(r pgx.Row) (APIToken, error) {
	var t APIToken
	err := r.Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt, &t.LastUsedAt, &t.ExpiresAt, &t.RevokedAt)
	return t, notFound(err)
}

// CreateToken issues a token; expires may be nil for no expiry.
func (s *Store) CreateToken(ctx context.Context, user, name string, expires *time.Time) (string, APIToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", APIToken{}, err
	}
	secret := TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	t, err := scanToken(s.Pool.QueryRow(ctx, `INSERT INTO api_tokens(id,user_id,name,token_hash,expires_at) VALUES($1,$2,$3,$4,$5) RETURNING `+tokenColumns,
		NewID("tok_"), user, name, Hash(secret), expires))
	return secret, t, err
}

func (s *Store) Tokens(ctx context.Context, user string) ([]APIToken, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+tokenColumns+` FROM api_tokens WHERE user_id=$1 ORDER BY created_at DESC`, user)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (APIToken, error) { return scanToken(r) })
}

// RevokeToken revokes one of the user's tokens.
func (s *Store) RevokeToken(ctx context.Context, user, id string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, user)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// ResolveToken returns the user of a live token and records its use.
func (s *Store) ResolveToken(ctx context.Context, secret string) (APIToken, User, error) {
	t, err := scanToken(s.Pool.QueryRow(ctx, `UPDATE api_tokens SET last_used_at=now() WHERE token_hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())
RETURNING `+tokenColumns, Hash(secret)))
	if err != nil {
		return t, User{}, err
	}
	u, err := s.User(ctx, t.UserID)
	return t, u, err
}
