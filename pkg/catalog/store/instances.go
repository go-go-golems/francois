package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

type Instance struct {
	ID          string    `json:"id"`
	Hostname    string    `json:"hostname"`
	OwnerUserID string    `json:"ownerUserId"`
	ReleaseID   string    `json:"releaseId"`
	Title       string    `json:"title"`
	Kind        string    `json:"kind"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	// ExpiresAt is set for demo instances.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// Role is the requesting user's effective membership role, when scoped.
	Role string `json:"role,omitempty"`
	// Denormalized display fields.
	ArtifactID string `json:"artifactId"`
	Version    string `json:"version"`
	OwnerName  string `json:"ownerName"`
	Members    int    `json:"members"`
}

const instanceSelect = `SELECT i.id,i.hostname,i.owner_user_id,i.release_id,i.title,i.kind,i.state,i.created_at,i.updated_at,i.expires_at,
 r.artifact_id,r.version,COALESCE(NULLIF(o.display_name,''),o.email),
 (SELECT count(*) FROM instance_members m2 WHERE m2.instance_id=i.id)`

func scanInstance(row pgx.Row, withRole bool) (Instance, error) {
	var i Instance
	dest := []any{&i.ID, &i.Hostname, &i.OwnerUserID, &i.ReleaseID, &i.Title, &i.Kind, &i.State, &i.CreatedAt, &i.UpdatedAt, &i.ExpiresAt, &i.ArtifactID, &i.Version, &i.OwnerName, &i.Members}
	if withRole {
		dest = append(dest, &i.Role)
	}
	return i, notFound(row.Scan(dest...))
}

// CreateInstance creates a private instance with its owner membership.
func (s *Store) CreateInstance(ctx context.Context, owner, release, title, kind string, limit int) (Instance, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Instance{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize per-owner admission so concurrent creates respect the limit.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('instances:'||$1))`, owner); err != nil {
		return Instance{}, err
	}
	if kind == "workspace" && limit > 0 {
		var n int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM instances WHERE owner_user_id=$1 AND kind='workspace'`, owner).Scan(&n); err != nil {
			return Instance{}, err
		}
		if n >= limit {
			return Instance{}, errors.Wrap(ErrLimit, "instance limit reached")
		}
	}
	hex := RandomHex(10)
	id := "ins_" + hex
	if _, err = tx.Exec(ctx, `INSERT INTO instances(id,hostname,owner_user_id,release_id,title,kind) VALUES($1,$2,$3,$4,$5,$6)`, id, "i-"+hex, owner, release, title, kind); err != nil {
		return Instance{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO instance_members(instance_id,user_id,role,added_by) VALUES($1,$2,'owner',$2)`, id, owner); err != nil {
		return Instance{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Instance{}, err
	}
	return s.MemberInstance(ctx, owner, id)
}

// WorkspacePosition is the 1-based position of a workspace among its
// owner's workspaces, oldest first. After a downgrade the oldest workspaces
// up to the plan's limit keep working.
func (s *Store) WorkspacePosition(ctx context.Context, inst Instance) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM instances WHERE owner_user_id=$1 AND kind='workspace' AND (created_at, id) <= ($2, $3)`,
		inst.OwnerUserID, inst.CreatedAt, inst.ID).Scan(&n)
	return n, err
}

// ErrLimit signals a configured quota.
var ErrLimit = errors.New("limit")

// MemberInstances lists instances the user belongs to, with the user's role.
func (s *Store) MemberInstances(ctx context.Context, user string) ([]Instance, error) {
	rows, err := s.Pool.Query(ctx, instanceSelect+`,m.role FROM instances i JOIN instance_members m ON m.instance_id=i.id AND m.user_id=$1
 JOIN releases r ON r.id=i.release_id JOIN users o ON o.id=i.owner_user_id WHERE i.kind='workspace' ORDER BY i.updated_at DESC`, user)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Instance, error) { return scanInstance(r, true) })
}

// MemberInstance returns an instance only if the user is a member.
func (s *Store) MemberInstance(ctx context.Context, user, id string) (Instance, error) {
	return scanInstance(s.Pool.QueryRow(ctx, instanceSelect+`,m.role FROM instances i JOIN instance_members m ON m.instance_id=i.id AND m.user_id=$1
 JOIN releases r ON r.id=i.release_id JOIN users o ON o.id=i.owner_user_id WHERE i.id=$2`, user, id), true)
}

func (s *Store) Instance(ctx context.Context, id string) (Instance, error) {
	return scanInstance(s.Pool.QueryRow(ctx, instanceSelect+` FROM instances i JOIN releases r ON r.id=i.release_id JOIN users o ON o.id=i.owner_user_id WHERE i.id=$1`, id), false)
}

func (s *Store) InstanceByHost(ctx context.Context, host string) (Instance, error) {
	return scanInstance(s.Pool.QueryRow(ctx, instanceSelect+` FROM instances i JOIN releases r ON r.id=i.release_id JOIN users o ON o.id=i.owner_user_id WHERE i.hostname=$1`, host), false)
}

// AllInstances is the administrator listing.
func (s *Store) AllInstances(ctx context.Context, search string, limit int) ([]Instance, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.Pool.Query(ctx, instanceSelect+` FROM instances i JOIN releases r ON r.id=i.release_id JOIN users o ON o.id=i.owner_user_id
 WHERE $1='' OR i.id=$1 OR i.owner_user_id=$1 OR i.title ILIKE '%'||$1||'%' OR o.email ILIKE '%'||$1||'%' OR r.artifact_id=$1
 ORDER BY i.updated_at DESC LIMIT $2`, search, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Instance, error) { return scanInstance(r, false) })
}

func (s *Store) PreviewInstance(ctx context.Context, admin, release string) (Instance, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `SELECT id FROM instances WHERE owner_user_id=$1 AND release_id=$2 AND kind='preview'`, admin, release).Scan(&id)
	if err != nil {
		return Instance{}, notFound(err)
	}
	return s.MemberInstance(ctx, admin, id)
}

type Member struct {
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) Members(ctx context.Context, instance string) ([]Member, error) {
	rows, err := s.Pool.Query(ctx, `SELECT m.user_id,u.display_name,u.email,m.role,m.created_at FROM instance_members m JOIN users u ON u.id=m.user_id WHERE m.instance_id=$1 ORDER BY m.role='owner' DESC, m.created_at`, instance)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Member, error) {
		var m Member
		err := r.Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.CreatedAt)
		return m, err
	})
}

// SetMember adds or changes a non-owner membership within the member limit.
func (s *Store) SetMember(ctx context.Context, instance, user, role, actor string, limit int) error {
	if role != "editor" && role != "viewer" {
		return errors.New("role must be editor or viewer")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT 1 FROM instances WHERE id=$1 FOR UPDATE`, instance); err != nil {
		return err
	}
	var current string
	err = tx.QueryRow(ctx, `SELECT role FROM instance_members WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&current)
	switch {
	case err == nil && current == "owner":
		return errors.Wrap(ErrConflict, "the owner's role cannot change")
	case errors.Is(err, pgx.ErrNoRows):
		var n int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM instance_members WHERE instance_id=$1`, instance).Scan(&n); err != nil {
			return err
		}
		if n >= limit {
			return errors.Wrap(ErrLimit, "member limit reached")
		}
	case err != nil:
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO instance_members(instance_id,user_id,role,added_by) VALUES($1,$2,$3,$4) ON CONFLICT(instance_id,user_id) DO UPDATE SET role=EXCLUDED.role`, instance, user, role, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveMember removes a non-owner member and revokes their grants.
func (s *Store) RemoveMember(ctx context.Context, instance, user string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `DELETE FROM instance_members WHERE instance_id=$1 AND user_id=$2 AND role<>'owner'`, instance, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE instance_id=$1 AND user_id=$2 AND revoked_at IS NULL`, instance, user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SetInstanceState(ctx context.Context, id, state string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE instances SET state=$2,updated_at=now() WHERE id=$1`, id, state)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if state == "suspended" {
		if _, err = tx.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE instance_id=$1 AND revoked_at IS NULL`, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// UpgradeInstance repins an instance to another published release of the
// same artifact, preserving identity, origin and data.
func (s *Store) UpgradeInstance(ctx context.Context, id, release string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE instances i SET release_id=$2,updated_at=now() FROM releases cur, releases nxt
WHERE i.id=$1 AND cur.id=i.release_id AND nxt.id=$2 AND nxt.artifact_id=cur.artifact_id AND nxt.state='published'`, id, release)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.Wrap(ErrConflict, "upgrade requires a published release of the same application")
	}
	return nil
}

func (s *Store) DeleteInstance(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM instances WHERE id=$1`, id)
	return err
}

// InstanceUsage summarizes durable state for administrators.
type InstanceUsage struct {
	Documents     int   `json:"documents"`
	DocumentBytes int64 `json:"documentBytes"`
	Assets        int   `json:"assets"`
	AssetBytes    int64 `json:"assetBytes"`
	ActiveGrants  int   `json:"activeGrants"`
}

func (s *Store) InstanceUsage(ctx context.Context, id string) (InstanceUsage, error) {
	var u InstanceUsage
	err := s.Pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM instance_documents WHERE instance_id=$1),
 (SELECT COALESCE(sum(size),0) FROM instance_documents WHERE instance_id=$1),
 (SELECT count(*) FROM instance_assets WHERE instance_id=$1),
 (SELECT COALESCE(sum(length(bytes)),0) FROM instance_assets WHERE instance_id=$1),
 (SELECT count(*) FROM runtime_grants WHERE instance_id=$1 AND revoked_at IS NULL AND expires_at>now())`, id).Scan(&u.Documents, &u.DocumentBytes, &u.Assets, &u.AssetBytes, &u.ActiveGrants)
	return u, err
}

// Launch codes and runtime grants.

type LaunchCode struct {
	SessionHash []byte
	UserID      string
	InstanceID  string
	Origin      string
	Challenge   string
}

func (s *Store) CreateLaunchCode(ctx context.Context, c LaunchCode, ttl time.Duration) (string, error) {
	code := RandomHex(24)
	_, err := s.Pool.Exec(ctx, `INSERT INTO launch_codes(code_hash,session_hash,user_id,instance_id,origin,challenge,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		Hash(code), c.SessionHash, c.UserID, c.InstanceID, c.Origin, c.Challenge, time.Now().Add(ttl))
	if err == nil {
		_, _ = s.Pool.Exec(ctx, `DELETE FROM launch_codes WHERE expires_at < now() - interval '1 hour'`)
	}
	return code, err
}

// ConsumeLaunchCode atomically consumes an unexpired code for an origin and
// returns its binding. The caller verifies the PKCE verifier.
func (s *Store) ConsumeLaunchCode(ctx context.Context, code, origin string) (LaunchCode, error) {
	var c LaunchCode
	err := s.Pool.QueryRow(ctx, `UPDATE launch_codes SET consumed_at=now() WHERE code_hash=$1 AND origin=$2 AND consumed_at IS NULL AND expires_at>now()
RETURNING session_hash,user_id,instance_id,origin,challenge`, Hash(code), origin).Scan(&c.SessionHash, &c.UserID, &c.InstanceID, &c.Origin, &c.Challenge)
	return c, notFound(err)
}

type Grant struct {
	SessionHash []byte
	UserID      string
	InstanceID  string
	ExpiresAt   time.Time
}

// CreateGrant issues an instance-scoped bearer grant derived from a live
// portal session; revoking the session revokes the grant.
func (s *Store) CreateGrant(ctx context.Context, g Grant) (string, error) {
	token := RandomHex(32)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var live bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now())`, g.SessionHash).Scan(&live); err != nil {
		return "", err
	}
	if !live {
		return "", ErrNotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO runtime_grants(token_hash,session_hash,user_id,instance_id,expires_at) VALUES($1,$2,$3,$4,$5)`, Hash(token), g.SessionHash, g.UserID, g.InstanceID, g.ExpiresAt); err != nil {
		return "", err
	}
	_, _ = tx.Exec(ctx, `DELETE FROM runtime_grants WHERE expires_at < now() - interval '1 day'`)
	return token, tx.Commit(ctx)
}

// ResolveGrant validates a grant for a given instance; it also requires the
// originating portal session to remain live.
func (s *Store) ResolveGrant(ctx context.Context, token, instance string) (Grant, error) {
	var g Grant
	err := s.Pool.QueryRow(ctx, `SELECT g.session_hash,g.user_id,g.instance_id,g.expires_at FROM runtime_grants g JOIN sessions s ON s.token_hash=g.session_hash
WHERE g.token_hash=$1 AND g.instance_id=$2 AND g.revoked_at IS NULL AND g.expires_at>now() AND s.revoked_at IS NULL AND s.expires_at>now()`, Hash(token), instance).Scan(&g.SessionHash, &g.UserID, &g.InstanceID, &g.ExpiresAt)
	return g, notFound(err)
}
