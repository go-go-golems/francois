package store

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

// GuestIssuer marks demo visitors' accounts; they never sign in through the
// identity provider.
const GuestIssuer = "urn:francois:guest"

// CreateGuest creates a demo visitor account.
func (s *Store) CreateGuest(ctx context.Context) (User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `INSERT INTO users(id,issuer,subject,display_name,role) VALUES($1,$2,$3,'Guest','guest') RETURNING `+userColumns,
		NewID("usr_"), GuestIssuer, RandomHex(16)))
}

// CreateGuestSession is a portal session that ends after ttl, sliding or not.
func (s *Store) CreateGuestSession(ctx context.Context, user string, ttl time.Duration) (string, Session, error) {
	token := RandomHex(32)
	end := time.Now().Add(ttl)
	sess := Session{Hash: Hash(token), UserID: user, CSRF: RandomHex(24), ExpiresAt: end}
	_, err := s.Pool.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at,id_token) VALUES($1,$2,$3,$4,$4,'')`,
		sess.Hash, user, sess.CSRF, end)
	return token, sess, err
}

// ErrDemoCapacity reports that the maximum number of live demos is reached.
var ErrDemoCapacity = errors.New("demo capacity reached")

// CreateDemo creates a demo instance that expires at expires, unless limit
// live demos already exist (admission is serialized so concurrent visitors
// cannot exceed it).
func (s *Store) CreateDemo(ctx context.Context, owner, release, title string, expires time.Time, limit int) (Instance, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Instance{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('demos'))`); err != nil {
		return Instance{}, err
	}
	var live int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM instances WHERE kind='demo' AND state='active' AND expires_at > now()`).Scan(&live); err != nil {
		return Instance{}, err
	}
	if live >= limit {
		return Instance{}, ErrDemoCapacity
	}
	hex := RandomHex(10)
	id := "ins_" + hex
	if _, err = tx.Exec(ctx, `INSERT INTO instances(id,hostname,owner_user_id,release_id,title,kind,expires_at) VALUES($1,$2,$3,$4,$5,'demo',$6)`,
		id, "i-"+hex, owner, release, title, expires); err != nil {
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

// ExpireDemos suspends demo instances past their expiry (returning their
// IDs so live views can be closed) and deletes those expired longer than
// keep ago. Their model calls stay for the daily demo budget and usage.
func (s *Store) ExpireDemos(ctx context.Context, keep time.Duration) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `UPDATE instances SET state='suspended',updated_at=now() WHERE kind='demo' AND state='active' AND expires_at <= now() RETURNING id`)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if _, err = s.Pool.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE revoked_at IS NULL AND instance_id = ANY($1)`, ids); err != nil {
		return nil, err
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM instances WHERE kind='demo' AND expires_at < $1`, time.Now().Add(-keep))
	return ids, err
}
