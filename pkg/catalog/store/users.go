package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

type User struct {
	ID            string     `json:"id"`
	Issuer        string     `json:"issuer"`
	Subject       string     `json:"subject"`
	Email         string     `json:"email"`
	EmailVerified bool       `json:"emailVerified"`
	DisplayName   string     `json:"name"`
	Role          string     `json:"role"`
	Disabled      bool       `json:"disabled"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
}

const userColumns = `id,issuer,subject,email,email_verified,display_name,role,disabled,created_at,last_login_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.EmailVerified, &u.DisplayName, &u.Role, &u.Disabled, &u.CreatedAt, &u.LastLoginAt)
	return u, notFound(err)
}

// Identity holds verified OIDC claims; email and name are mutable profile data.
type Identity struct {
	Issuer, Subject, Email, Name string
	EmailVerified                bool
}

// UpsertIdentity provisions or refreshes a local user keyed by issuer and
// subject. Role and disabled flags are never changed by a login.
func (s *Store) UpsertIdentity(ctx context.Context, id Identity) (User, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Missing email is not an address: guests and OIDC identities may all
	// have an empty placeholder. Coordinate ownership only for actual email.
	if id.Email != "" {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1),0))`, id.Email); err != nil {
			return User{}, err
		}
		// A signup may have committed while this OIDC callback waited for the
		// email lock. Neither a new identity nor an existing identity's profile
		// refresh may claim an address belonging to a different issuer/subject.
		var collision bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1) AND (issuer<>$2 OR subject<>$3))`, id.Email, id.Issuer, id.Subject).Scan(&collision)
		if err != nil {
			return User{}, err
		}
		if collision {
			return User{}, ErrNotFound
		}
	}
	u, err := scanUser(tx.QueryRow(ctx, `INSERT INTO users(id,issuer,subject,email,email_verified,display_name,last_login_at)
VALUES($1,$2,$3,$4,$5,$6,now())
ON CONFLICT(issuer,subject) DO UPDATE SET email=EXCLUDED.email,email_verified=EXCLUDED.email_verified,display_name=EXCLUDED.display_name,last_login_at=now()
RETURNING `+userColumns, NewID("usr_"), id.Issuer, id.Subject, id.Email, id.EmailVerified, id.Name))
	if err != nil {
		return User{}, err
	}
	return u, tx.Commit(ctx)
}

func (s *Store) User(ctx context.Context, id string) (User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1`, id))
}

func (s *Store) UserByIdentity(ctx context.Context, issuer, subject string) (User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE issuer=$1 AND subject=$2`, issuer, subject))
}

// UserByVerifiedEmail finds exactly one registered user with a verified address.
func (s *Store) UserByVerifiedEmail(ctx context.Context, email string) (User, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email)=lower($1) AND email_verified AND NOT disabled LIMIT 2`, email)
	if err != nil {
		return User{}, err
	}
	users, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
	if err != nil {
		return User{}, err
	}
	if len(users) != 1 {
		return User{}, ErrNotFound
	}
	return users[0], nil
}

// Users lists accounts for administrators; demo guests are left out.
func (s *Store) Users(ctx context.Context, search string, limit int) ([]User, error) {
	return s.UserPage(ctx, UserFilter{Search: search, Limit: limit})
}

// UserFilter selects non-guest users, newest first. Role "" matches every
// role; Offset pages through the result.
type UserFilter struct {
	Search, Role  string
	Limit, Offset int
}

func (s *Store) UserPage(ctx context.Context, f UserFilter) ([]User, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+userColumns+` FROM users
WHERE role<>'guest' AND ($1='' OR email ILIKE '%'||$1||'%' OR display_name ILIKE '%'||$1||'%' OR id=$1 OR subject=$1) AND ($2='' OR role=$2)
ORDER BY created_at DESC, id LIMIT $3 OFFSET $4`, f.Search, f.Role, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
}

func (s *Store) SetUserRole(ctx context.Context, id, role string) error {
	// guest is assigned only by the server to demo visitors.
	if role != "subscriber" && role != "creator" && role != "admin" {
		return errors.New("role must be subscriber, creator or admin")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var old string
	if err = tx.QueryRow(ctx, `SELECT role FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&old); err != nil {
		return notFound(err)
	}
	if old == role {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET role=$2 WHERE id=$1`, id, role); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=now(),id_token='' WHERE user_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM email_links WHERE (issuer,subject)=(SELECT issuer,subject FROM users WHERE id=$1)`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetUserDisabled toggles hosted access. Disabling revokes runtime grants;
// sessions stay usable for restricted billing management.
func (s *Store) SetUserDisabled(ctx context.Context, id string, disabled bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE users SET disabled=$2 WHERE id=$1`, id, disabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if disabled {
		if _, err = tx.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM email_links WHERE (issuer,subject)=(SELECT issuer,subject FROM users WHERE id=$1)`, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Audit records an administrative action with a nonsecret summary.
func (s *Store) Audit(ctx context.Context, actor, action, subject string, summary map[string]any) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	var a any
	if actor != "" {
		a = actor
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO admin_audit(actor_id,token_id,action,subject,summary) VALUES($1,$2,$3,$4,$5)`, a, auditToken(ctx), action, subject, raw)
	return err
}

type AuditEntry struct {
	ID        int64           `json:"id"`
	ActorID   *string         `json:"actorId"`
	TokenID   *string         `json:"tokenId,omitempty"`
	Action    string          `json:"action"`
	Subject   string          `json:"subject"`
	Summary   json.RawMessage `json:"summary"`
	CreatedAt time.Time       `json:"createdAt"`
}

func (s *Store) AuditLog(ctx context.Context, subject string, limit int) ([]AuditEntry, error) {
	return s.AuditQuery(ctx, AuditFilter{Subject: subject, Limit: limit})
}

// AuditFilter selects audit entries; empty fields match everything. Before
// pages backwards: entries with an ID below it.
type AuditFilter struct {
	Subject, Actor, Action string
	Before                 int64
	Limit                  int
}

func (s *Store) AuditQuery(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := s.Pool.Query(ctx, `SELECT id,actor_id,token_id,action,subject,summary,created_at FROM admin_audit
WHERE ($1='' OR subject=$1) AND ($2='' OR actor_id=$2) AND ($3='' OR action=$3) AND ($4=0 OR id<$4)
ORDER BY id DESC LIMIT $5`, f.Subject, f.Actor, f.Action, f.Before, f.Limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AuditEntry, error) {
		var e AuditEntry
		err := r.Scan(&e.ID, &e.ActorID, &e.TokenID, &e.Action, &e.Subject, &e.Summary, &e.CreatedAt)
		return e, err
	})
}

type AccessGrant struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Until     time.Time  `json:"until"`
	Reason    string     `json:"reason"`
	GrantedBy string     `json:"grantedBy"`
	CreatedAt time.Time  `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	Plan      string     `json:"plan"`
}

// GrantAccess records complimentary hosted access; it never fabricates a
// Stripe subscription.
// plan names the granted plan; empty means the highest plan.
func (s *Store) GrantAccess(ctx context.Context, user, actor string, until time.Time, reason, plan string) (AccessGrant, error) {
	g := AccessGrant{ID: NewID("agr_"), UserID: user, Until: until.UTC(), Reason: reason, GrantedBy: actor, Plan: plan}
	err := s.Pool.QueryRow(ctx, `INSERT INTO access_grants(id,user_id,until,reason,granted_by,plan) VALUES($1,$2,$3,$4,$5,$6) RETURNING created_at`, g.ID, user, g.Until, reason, actor, plan).Scan(&g.CreatedAt)
	return g, err
}

// RevokeAccessGrant revokes one live grant and returns it.
func (s *Store) RevokeAccessGrant(ctx context.Context, id string) (AccessGrant, error) {
	var g AccessGrant
	err := s.Pool.QueryRow(ctx, `UPDATE access_grants SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL
RETURNING id,user_id,until,reason,granted_by,created_at,revoked_at,plan`, id).Scan(&g.ID, &g.UserID, &g.Until, &g.Reason, &g.GrantedBy, &g.CreatedAt, &g.RevokedAt, &g.Plan)
	return g, notFound(err)
}

func (s *Store) RevokeAccessGrants(ctx context.Context, user string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE access_grants SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, user)
	return err
}

func (s *Store) AccessGrants(ctx context.Context, user string) ([]AccessGrant, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,user_id,until,reason,granted_by,created_at,revoked_at,plan FROM access_grants WHERE user_id=$1 ORDER BY created_at DESC`, user)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AccessGrant, error) {
		var g AccessGrant
		err := r.Scan(&g.ID, &g.UserID, &g.Until, &g.Reason, &g.GrantedBy, &g.CreatedAt, &g.RevokedAt, &g.Plan)
		return g, err
	})
}
