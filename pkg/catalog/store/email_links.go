package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const EmailLinkTTL = 15 * time.Minute

type EmailLink struct {
	Issuer, Subject, Email, Role, Origin, ReturnPath string
	CreatedAt, ExpiresAt                             time.Time
}

// UsersByEmail includes disabled/unverified records; neither means absent.
func (s *Store) UsersByEmail(ctx context.Context, email string) ([]User, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email)=lower($1) LIMIT 2`, email)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
}

func (s *Store) PrepareEmailLink(ctx context.Context, token string, binding []byte, id Identity, signup bool, origin, ret string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1),0))`, id.Email); err != nil {
		return err
	}
	var collision bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1) AND (issuer<>$2 OR subject<>$3))`, id.Email, id.Issuer, id.Subject).Scan(&collision); err != nil {
		return err
	}
	if collision {
		return ErrNotFound
	}
	u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE issuer=$1 AND subject=$2 FOR UPDATE`, id.Issuer, id.Subject))
	role := "subscriber"
	if err == ErrNotFound {
		if !signup {
			return ErrNotFound
		}
	} else if err != nil {
		return err
	} else {
		if u.Disabled || u.Role == "guest" {
			return ErrNotFound
		}
		role = u.Role
	}
	if _, err = tx.Exec(ctx, `DELETE FROM email_links WHERE expires_at<now()`); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO email_links(flow_hash,binding_hash,issuer,subject,email,role,portal_origin,return_path,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now()+interval '15 minutes')`, Hash(token), binding, id.Issuer, id.Subject, id.Email, role, origin, ret)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SetEmailLinkReady(ctx context.Context, token string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE email_links SET ready=true WHERE flow_hash=$1 AND expires_at>now()`, Hash(token))
	if err == nil && tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return err
}

func (s *Store) DiscardEmailLink(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM email_links WHERE flow_hash=$1`, Hash(token))
	return err
}

func (s *Store) EmailLinkReady(ctx context.Context, token string, binding []byte, origin string) (bool, error) {
	var ready bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM email_links WHERE flow_hash=$1 AND binding_hash=$2 AND portal_origin=$3 AND ready AND expires_at>now())`, Hash(token), binding, origin).Scan(&ready)
	return ready, err
}

func (s *Store) ClaimEmailLink(ctx context.Context, token string, binding []byte, origin string) (EmailLink, error) {
	var f EmailLink
	err := s.Pool.QueryRow(ctx, `DELETE FROM email_links WHERE flow_hash=$1 AND binding_hash=$2 AND portal_origin=$3 AND ready AND expires_at>now() RETURNING issuer,subject,email,role,portal_origin,return_path,created_at,expires_at`, Hash(token), binding, origin).Scan(&f.Issuer, &f.Subject, &f.Email, &f.Role, &f.Origin, &f.ReturnPath, &f.CreatedAt, &f.ExpiresAt)
	return f, notFound(err)
}

// CompleteEmailLink reconciles only the proven issuer/subject. An account
// provisioned by SSO while the link was pending is the same account, not a clash.
func (s *Store) CompleteEmailLink(ctx context.Context, f EmailLink, id Identity, signup bool) (string, error) {
	if !id.EmailVerified || id.Issuer != f.Issuer || id.Subject != f.Subject || !strings.EqualFold(id.Email, f.Email) {
		return "", ErrNotFound
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1),0))`, id.Email); err != nil {
		return "", err
	}
	if !f.ExpiresAt.After(time.Now()) {
		return "", ErrNotFound
	}
	var collision bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1) AND (issuer<>$2 OR subject<>$3))`, id.Email, id.Issuer, id.Subject).Scan(&collision); err != nil {
		return "", err
	}
	if collision {
		return "", ErrNotFound
	}
	u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE issuer=$1 AND subject=$2 FOR UPDATE`, id.Issuer, id.Subject))
	switch err {
	case ErrNotFound:
		if !signup || f.Role != "subscriber" {
			return "", ErrNotFound
		}
		u, err = scanUser(tx.QueryRow(ctx, `INSERT INTO users(id,issuer,subject,email,email_verified,display_name,last_login_at) VALUES($1,$2,$3,$4,true,$5,now()) RETURNING `+userColumns, NewID("usr_"), id.Issuer, id.Subject, id.Email, id.Name))
	case nil:
		if u.Disabled || u.Role == "guest" || u.Role != f.Role {
			return "", ErrNotFound
		}
		_, err = tx.Exec(ctx, `UPDATE users SET email=$2,email_verified=true,display_name=CASE WHEN $3='' THEN display_name ELSE $3 END,last_login_at=now() WHERE id=$1`, u.ID, id.Email, id.Name)
	}
	if err != nil {
		return "", err
	}
	token, _, err := insertSession(ctx, tx, u.ID, "")
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM email_links WHERE issuer=$1 AND subject=$2`, id.Issuer, id.Subject); err != nil {
		return "", err
	}
	return token, tx.Commit(ctx)
}
