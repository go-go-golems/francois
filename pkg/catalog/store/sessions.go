package store

import (
	"context"
	"time"
)

type LoginFlow struct {
	BindingHash []byte
	Nonce       string
	Verifier    string
	ReturnPath  string
}

func (s *Store) SaveLoginFlow(ctx context.Context, state string, f LoginFlow, ttl time.Duration) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM login_flows WHERE expires_at < now() - interval '1 hour'`)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO login_flows(state_hash,binding_hash,nonce,verifier,return_path,expires_at) VALUES($1,$2,$3,$4,$5,$6)`,
		Hash(state), f.BindingHash, f.Nonce, f.Verifier, f.ReturnPath, time.Now().Add(ttl))
	return err
}

// ConsumeLoginFlow atomically consumes an unexpired flow bound to the browser.
func (s *Store) ConsumeLoginFlow(ctx context.Context, state string, binding []byte) (LoginFlow, error) {
	var f LoginFlow
	err := s.Pool.QueryRow(ctx, `UPDATE login_flows SET consumed_at=now()
WHERE state_hash=$1 AND binding_hash=$2 AND consumed_at IS NULL AND expires_at > now()
RETURNING binding_hash,nonce,verifier,return_path`, Hash(state), binding).Scan(&f.BindingHash, &f.Nonce, &f.Verifier, &f.ReturnPath)
	return f, notFound(err)
}

type Session struct {
	Hash      []byte
	UserID    string
	CSRF      string
	ExpiresAt time.Time
}

const (
	SessionIdle     = 7 * 24 * time.Hour
	SessionAbsolute = 30 * 24 * time.Hour
)

// CreateSession stores only the digest of a fresh random token.
func (s *Store) CreateSession(ctx context.Context, user string) (string, Session, error) {
	token := RandomHex(32)
	sess := Session{Hash: Hash(token), UserID: user, CSRF: RandomHex(24), ExpiresAt: time.Now().Add(SessionIdle)}
	_, err := s.Pool.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at) VALUES($1,$2,$3,$4,$5)`,
		sess.Hash, user, sess.CSRF, sess.ExpiresAt, time.Now().Add(SessionAbsolute))
	return token, sess, err
}

// ResolveSession returns a usable session and slides its idle expiry.
func (s *Store) ResolveSession(ctx context.Context, token string) (Session, User, error) {
	var sess Session
	row := s.Pool.QueryRow(ctx, `UPDATE sessions SET last_seen_at=now(), expires_at=LEAST(absolute_expires_at, now()+$2::interval)
WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at > now() AND absolute_expires_at > now()
RETURNING token_hash,user_id,csrf_token,expires_at`, Hash(token), SessionIdle.String())
	if err := row.Scan(&sess.Hash, &sess.UserID, &sess.CSRF, &sess.ExpiresAt); err != nil {
		return sess, User{}, notFound(err)
	}
	u, err := s.User(ctx, sess.UserID)
	return sess, u, err
}

// RevokeSession revokes one session and every runtime grant derived from it.
func (s *Store) RevokeSession(ctx context.Context, hash []byte) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE session_hash=$1 AND revoked_at IS NULL`, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RevokeUserSessions revokes every portal session and runtime grant of a user.
func (s *Store) RevokeUserSessions(ctx context.Context, user string) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, user)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, user); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), tx.Commit(ctx)
}

func (s *Store) ActiveSessions(ctx context.Context, user string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at > now()`, user).Scan(&n)
	return n, err
}
