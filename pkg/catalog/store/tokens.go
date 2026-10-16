package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

// TokenPrefix marks personal API tokens.
const TokenPrefix = "frt_"

// Token scopes select the API surfaces a personal token may call. A scope is
// a ceiling: the owner's role and the per-object checks still apply.
const (
	ScopeStudio     = "studio"      // /api/studio/*
	ScopeAccount    = "account"     // the owner's account, billing links and instances
	ScopeAdminRead  = "admin:read"  // GET /api/admin/*
	ScopeAdminWrite = "admin:write" // every method under /api/admin/*; implies admin:read
)

// ScopesForRole lists the scopes a user with this role may put on a token.
func ScopesForRole(role string) []string {
	switch role {
	case "admin":
		return []string{ScopeStudio, ScopeAccount, ScopeAdminRead, ScopeAdminWrite}
	case "creator":
		return []string{ScopeStudio, ScopeAccount}
	case "subscriber":
		return []string{ScopeAccount}
	default:
		return nil
	}
}

// IsAdminScope reports whether a scope reaches the admin API.
func IsAdminScope(scope string) bool { return scope == ScopeAdminRead || scope == ScopeAdminWrite }

// APIToken is a personal API token. Only its hash is stored; the secret is
// shown once at creation.
type APIToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

// Allows reports whether the token carries scope need; admin:write implies
// admin:read.
func (t APIToken) Allows(need string) bool {
	return slices.Contains(t.Scopes, need) || (need == ScopeAdminRead && slices.Contains(t.Scopes, ScopeAdminWrite))
}

const tokenColumns = `id,user_id,name,scopes,created_at,last_used_at,expires_at,revoked_at`

func scanToken(r pgx.Row) (APIToken, error) {
	var t APIToken
	err := r.Scan(&t.ID, &t.UserID, &t.Name, &t.Scopes, &t.CreatedAt, &t.LastUsedAt, &t.ExpiresAt, &t.RevokedAt)
	return t, notFound(err)
}

// CreateToken issues a token with the given scopes; expires may be nil for no
// expiry. Callers validate scopes against the owner's role.
func (s *Store) CreateToken(ctx context.Context, user, name string, scopes []string, expires *time.Time) (string, APIToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", APIToken{}, err
	}
	secret := TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	t, err := scanToken(s.Pool.QueryRow(ctx, `INSERT INTO api_tokens(id,user_id,name,scopes,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+tokenColumns,
		NewID("tok_"), user, name, scopes, Hash(secret), expires))
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

// RevokeTokenByID revokes any user's live token, for administrators, and
// returns it so the caller can audit against its owner.
func (s *Store) RevokeTokenByID(ctx context.Context, id string) (APIToken, error) {
	return scanToken(s.Pool.QueryRow(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL RETURNING `+tokenColumns, id))
}

// RevokeUserTokens revokes all of a user's live tokens.
func (s *Store) RevokeUserTokens(ctx context.Context, user string) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, user)
	return tag.RowsAffected(), err
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
