package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const EmailSigninTTL = 5 * time.Minute

type EmailSignin struct {
	UserID, Issuer, Subject, Email, Origin, ReturnPath, ProviderSession, Role string
	CreatedAt, ExpiresAt                                                      time.Time
}

// ReplaceEmailSignin holds the user lock only while replacing local state.
// Returning the displaced provider ID allows cleanup outside the transaction.
func (s *Store) ReplaceEmailSignin(ctx context.Context, token string, binding []byte, u User, origin, ret string) (EmailSignin, string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return EmailSignin{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1 FOR UPDATE`, u.ID))
	if err != nil {
		return EmailSignin{}, "", err
	}
	if current.Role != u.Role || !sameEmailUser(current, u.Issuer, u.Subject, u.Email) {
		return EmailSignin{}, "", ErrNotFound
	}
	var old *string
	err = tx.QueryRow(ctx, `DELETE FROM email_signins WHERE user_id=$1 RETURNING provider_session_id`, u.ID).Scan(&old)
	if err != nil && err != pgx.ErrNoRows {
		return EmailSignin{}, "", err
	}
	f := EmailSignin{UserID: u.ID, Issuer: u.Issuer, Subject: u.Subject, Email: u.Email, Origin: origin, ReturnPath: ret, Role: current.Role}
	err = tx.QueryRow(ctx, `INSERT INTO email_signins(flow_hash,user_id,binding_hash,issuer,subject,email_snapshot,portal_origin,return_path,expires_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,now()+interval '5 minutes') RETURNING created_at,expires_at`, Hash(token), u.ID, binding, u.Issuer, u.Subject, u.Email, origin, ret).Scan(&f.CreatedAt, &f.ExpiresAt)
	if err != nil {
		return f, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return f, "", err
	}
	if old != nil {
		return f, *old, nil
	}
	return f, "", nil
}

func sameEmailUser(u User, issuer, subject, email string) bool {
	return !u.Disabled && (u.Role == "subscriber" || u.Role == "creator" || u.Role == "admin") && u.EmailVerified && u.Issuer == issuer && u.Subject == subject && strings.EqualFold(u.Email, email)
}

func (s *Store) AttachEmailSession(ctx context.Context, token, provider string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE email_signins SET provider_session_id=$2 WHERE flow_hash=$1 AND expires_at>now() AND provider_session_id IS NULL`, Hash(token), provider)
	return tag.RowsAffected() == 1, err
}

// EmailSigninReady is a read-only check used by email-scanner GET requests.
func (s *Store) EmailSigninReady(ctx context.Context, token string, binding []byte, origin string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM email_signins WHERE flow_hash=$1 AND binding_hash=$2 AND portal_origin=$3 AND expires_at>now() AND provider_session_id IS NOT NULL)`, Hash(token), binding, origin).Scan(&ok)
	return ok, err
}

// ClaimEmailSignin permits exactly one verification attempt per issued link.
func (s *Store) ClaimEmailSignin(ctx context.Context, token string, binding []byte, origin string) (EmailSignin, error) {
	var f EmailSignin
	err := s.Pool.QueryRow(ctx, `DELETE FROM email_signins WHERE flow_hash=$1 AND binding_hash=$2 AND portal_origin=$3 AND expires_at>now() AND provider_session_id IS NOT NULL
 RETURNING user_id,issuer,subject,email_snapshot,portal_origin,return_path,provider_session_id,created_at,expires_at,(SELECT role FROM users WHERE id=email_signins.user_id)`, Hash(token), binding, origin).Scan(&f.UserID, &f.Issuer, &f.Subject, &f.Email, &f.Origin, &f.ReturnPath, &f.ProviderSession, &f.CreatedAt, &f.ExpiresAt, &f.Role)
	return f, notFound(err)
}

// CreateEmailSession rechecks the saved identity under the same user lock as
// role updates. A provider check never authorizes a newly privileged account.
func (s *Store) CreateEmailSession(ctx context.Context, f EmailSignin) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1 FOR UPDATE`, f.UserID))
	if err != nil {
		return "", err
	}
	if u.Role != f.Role || !sameEmailUser(u, f.Issuer, f.Subject, f.Email) || !f.ExpiresAt.After(time.Now()) {
		return "", ErrNotFound
	}
	token, _, err := insertSession(ctx, tx, u.ID, "")
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET last_login_at=now() WHERE id=$1`, u.ID); err != nil {
		return "", err
	}
	return token, tx.Commit(ctx)
}
