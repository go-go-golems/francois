package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// UsersByEmail deliberately includes disabled and unverified records: those
// must never be mistaken for an address eligible for a new registration.
func (s *Store) UsersByEmail(ctx context.Context, email string) ([]User, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email)=lower($1) LIMIT 2`, email)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
}

// ReplaceEmailRegistration reserves a stable subject before any provider I/O.
func (s *Store) ReplaceEmailRegistration(ctx context.Context, issuer, email, token string, binding []byte, origin, ret string) (EmailSignin, string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return EmailSignin{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO email_registrations(subject,issuer,email) VALUES($1,$2,$3) ON CONFLICT(issuer,email) DO NOTHING`, NewID("signup_"), issuer, strings.ToLower(email))
	if err != nil {
		return EmailSignin{}, "", err
	}
	var f EmailSignin
	var old string
	err = tx.QueryRow(ctx, `SELECT subject,provider_session_id FROM email_registrations WHERE issuer=$1 AND email=$2 FOR UPDATE`, issuer, strings.ToLower(email)).Scan(&f.Subject, &old)
	if err != nil {
		return f, "", err
	}
	f.Issuer, f.Email, f.Origin, f.ReturnPath = issuer, email, origin, ret
	err = tx.QueryRow(ctx, `UPDATE email_registrations SET flow_hash=$2,binding_hash=$3,portal_origin=$4,return_path=$5,provider_session_id='',ready=false,created_at=now(),expires_at=now()+interval '5 minutes' WHERE subject=$1 RETURNING created_at,expires_at`, f.Subject, Hash(token), binding, origin, ret).Scan(&f.CreatedAt, &f.ExpiresAt)
	if err != nil {
		return f, "", err
	}
	return f, old, tx.Commit(ctx)
}

func (s *Store) AttachEmailRegistration(ctx context.Context, token, session string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE email_registrations SET provider_session_id=$2,ready=true WHERE flow_hash=$1 AND expires_at>now() AND NOT ready`, Hash(token), session)
	return tag.RowsAffected() == 1, err
}

func (s *Store) EmailRegistrationReady(ctx context.Context, token string, binding []byte, origin string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM email_registrations WHERE flow_hash=$1 AND binding_hash=$2 AND portal_origin=$3 AND expires_at>now() AND ready)`, Hash(token), binding, origin).Scan(&ok)
	return ok, err
}

func (s *Store) ClaimEmailRegistration(ctx context.Context, token string, binding []byte, origin string) (EmailSignin, error) {
	var f EmailSignin
	err := s.Pool.QueryRow(ctx, `UPDATE email_registrations SET flow_hash=NULL,ready=false WHERE flow_hash=$1 AND binding_hash=$2 AND portal_origin=$3 AND expires_at>now() AND ready RETURNING issuer,subject,email,portal_origin,return_path,provider_session_id,created_at,expires_at`, Hash(token), binding, origin).Scan(&f.Issuer, &f.Subject, &f.Email, &f.Origin, &f.ReturnPath, &f.ProviderSession, &f.CreatedAt, &f.ExpiresAt)
	return f, notFound(err)
}

// CompleteEmailRegistration accepts a verified provider identity, not an email
// match. If OIDC already provisioned any account, use ordinary sign-in instead.
func (s *Store) CompleteEmailRegistration(ctx context.Context, f EmailSignin) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1),0))`, f.Email); err != nil {
		return "", err
	}
	if !f.ExpiresAt.After(time.Now()) {
		return "", ErrNotFound
	}
	var collision bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1) OR (issuer=$2 AND subject=$3))`, f.Email, f.Issuer, f.Subject).Scan(&collision)
	if err != nil {
		return "", err
	}
	if collision {
		return "", ErrNotFound
	}
	u, err := scanUser(tx.QueryRow(ctx, `INSERT INTO users(id,issuer,subject,email,email_verified,display_name,last_login_at)
VALUES($1,$2,$3,$4,true,'New user',now()) RETURNING `+userColumns, NewID("usr_"), f.Issuer, f.Subject, f.Email))
	if err != nil {
		return "", err
	}
	if !sameEmailUser(u, f.Issuer, f.Subject, f.Email) {
		return "", ErrNotFound
	}
	token, _, err := insertSession(ctx, tx, u.ID, "")
	if err != nil {
		return "", err
	}
	return token, tx.Commit(ctx)
}
