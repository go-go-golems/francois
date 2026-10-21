package store

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Device logins (RFC 8628 shaped): the CLI holds a secret device code and
// shows the human a short user code, which they approve in a signed-in
// browser. The CLI then claims a personal API token with the device code.

// DeviceTTL is how long a device login may wait for approval.
const DeviceTTL = 10 * time.Minute

// DeviceSlowDown is added to a device's polling interval each time it polls
// too fast; the interval never exceeds deviceMaxInterval.
const (
	DeviceSlowDown    = 5 * time.Second
	deviceMaxInterval = 60 * time.Second
)

// UserCodeAlphabet has no vowels (no accidental words) and no digits (no
// 0/O or 1/I confusion), as RFC 8628 §6.1 recommends.
const UserCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"

// NewUserCode returns 8 random letters from UserCodeAlphabet (~34.6 bits).
func NewUserCode() string {
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(UserCodeAlphabet))))
		if err != nil {
			panic(err)
		}
		b[i] = UserCodeAlphabet[n.Int64()]
	}
	return string(b)
}

// NormalizeUserCode accepts any spelling a human types ("wdjb-mjht",
// "WDJB MJHT") and returns the 8 letters, or false.
func NormalizeUserCode(in string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(in) {
		if strings.ContainsRune(UserCodeAlphabet, r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	return s, len(s) == 8
}

// FormatUserCode shows a normalized code as XXXX-XXXX.
func FormatUserCode(code string) string {
	if len(code) != 8 {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// DeviceAuthorization is a pending device login.
type DeviceAuthorization struct {
	Origin           string
	ClientName       string
	RequestedScopes  []string
	RequestedDays    int
	RequesterAddress string
	Interval         time.Duration
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

// CreateDeviceAuthorization stores a pending login under the hashes of both
// codes (userCode normalized) and drops logins that expired over an hour ago.
// ErrConflict means the user code is taken; pick another.
func (s *Store) CreateDeviceAuthorization(ctx context.Context, deviceCode, userCode string, d DeviceAuthorization) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM device_authorizations WHERE expires_at < now() - interval '1 hour'`); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO device_authorizations(device_code_hash,user_code_hash,portal_origin,client_name,requested_scopes,requested_days,requester_address,interval_seconds,expires_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,now()+$9::interval)`,
		Hash(deviceCode), Hash(userCode), d.Origin, d.ClientName, d.RequestedScopes, d.RequestedDays, d.RequesterAddress, int(d.Interval.Seconds()), DeviceTTL.String())
	if isUnique(err) {
		return ErrConflict
	}
	return err
}

// DeviceRequest returns the pending, unexpired login with this normalized
// user code on origin.
func (s *Store) DeviceRequest(ctx context.Context, userCode, origin string) (DeviceAuthorization, error) {
	d := DeviceAuthorization{Origin: origin}
	var interval int
	err := s.Pool.QueryRow(ctx, `SELECT client_name,requested_scopes,requested_days,requester_address,interval_seconds,created_at,expires_at FROM device_authorizations
WHERE user_code_hash=$1 AND portal_origin=$2 AND status='pending' AND expires_at > now()`, Hash(userCode), origin).
		Scan(&d.ClientName, &d.RequestedScopes, &d.RequestedDays, &d.RequesterAddress, &interval, &d.CreatedAt, &d.ExpiresAt)
	d.Interval = time.Duration(interval) * time.Second
	return d, notFound(err)
}

// ApproveDevice records who approved a pending login and what they granted.
// The token is minted when the CLI next polls.
func (s *Store) ApproveDevice(ctx context.Context, userCode, origin, user string, scopes []string, days int) error {
	return s.decideDevice(ctx, `UPDATE device_authorizations SET status='approved', user_id=$3, approved_scopes=$4, approved_days=$5
WHERE user_code_hash=$1 AND portal_origin=$2 AND status='pending' AND expires_at > now()`, Hash(userCode), origin, user, scopes, days)
}

// DenyDevice refuses a pending login.
func (s *Store) DenyDevice(ctx context.Context, userCode, origin, user string) error {
	return s.decideDevice(ctx, `UPDATE device_authorizations SET status='denied', user_id=$3
WHERE user_code_hash=$1 AND portal_origin=$2 AND status='pending' AND expires_at > now()`, Hash(userCode), origin, user)
}

func (s *Store) decideDevice(ctx context.Context, sql string, args ...any) error {
	tag, err := s.Pool.Exec(ctx, sql, args...)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Device poll outcomes.
const (
	DevicePending  = "pending"
	DeviceSlow     = "slow_down"
	DeviceDenied   = "denied"
	DeviceExpired  = "expired"
	DeviceIssued   = "issued"
	deviceApproved = "approved"
)

// DevicePoll is the answer to one poll. Secret, Token and User are set when
// State is DeviceIssued.
type DevicePoll struct {
	State    string
	Interval time.Duration
	Secret   string
	Token    APIToken
	User     User
}

// DeviceGrant re-checks an approved login against the approver as they are
// now and returns the token's scopes and expiry, or an error to deny it.
type DeviceGrant func(u User, scopes []string, days int) ([]string, *time.Time, error)

// PollDevice answers the CLI's poll for deviceCode on origin. For an approved
// login it re-checks the approver with grant, mints the token and deletes the
// login in one transaction, so a token is issued at most once and its secret
// is never stored.
func (s *Store) PollDevice(ctx context.Context, deviceCode, origin string, grant DeviceGrant) (DevicePoll, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return DevicePoll{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var (
		status, client string
		user           *string
		scopes         []string
		days           *int
		interval       int
		expired, fast  bool
	)
	err = tx.QueryRow(ctx, `SELECT status,user_id,client_name,approved_scopes,approved_days,interval_seconds,expires_at <= now(),
  last_polled_at IS NOT NULL AND last_polled_at > now() - make_interval(secs => interval_seconds)
FROM device_authorizations WHERE device_code_hash=$1 AND portal_origin=$2 FOR UPDATE`, Hash(deviceCode), origin).
		Scan(&status, &user, &client, &scopes, &days, &interval, &expired, &fast)
	if err == pgx.ErrNoRows || (err == nil && expired) {
		return DevicePoll{State: DeviceExpired}, nil
	}
	if err != nil {
		return DevicePoll{}, err
	}
	key := Hash(deviceCode)
	if fast {
		interval = min(interval+int(DeviceSlowDown.Seconds()), int(deviceMaxInterval.Seconds()))
		if _, err = tx.Exec(ctx, `UPDATE device_authorizations SET interval_seconds=$2, last_polled_at=now() WHERE device_code_hash=$1`, key, interval); err != nil {
			return DevicePoll{}, err
		}
		return DevicePoll{State: DeviceSlow, Interval: time.Duration(interval) * time.Second}, tx.Commit(ctx)
	}
	poll := DevicePoll{State: DevicePending, Interval: time.Duration(interval) * time.Second}
	switch status {
	case DevicePending:
		_, err = tx.Exec(ctx, `UPDATE device_authorizations SET last_polled_at=now() WHERE device_code_hash=$1`, key)
	case DeviceDenied:
		poll.State = DeviceDenied
		_, err = tx.Exec(ctx, `DELETE FROM device_authorizations WHERE device_code_hash=$1`, key)
	case deviceApproved:
		if user == nil || days == nil {
			return DevicePoll{}, ErrConflict
		}
		poll, err = s.issueDevice(ctx, tx, key, *user, client, scopes, *days, grant)
	}
	if err != nil {
		return DevicePoll{}, err
	}
	return poll, tx.Commit(ctx)
}

func (s *Store) issueDevice(ctx context.Context, tx pgx.Tx, key []byte, userID, client string, scopes []string, days int, grant DeviceGrant) (DevicePoll, error) {
	if _, err := tx.Exec(ctx, `DELETE FROM device_authorizations WHERE device_code_hash=$1`, key); err != nil {
		return DevicePoll{}, err
	}
	u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1`, userID))
	if err != nil {
		return DevicePoll{}, err
	}
	scopes, expires, err := grant(u, scopes, days)
	if err != nil {
		return DevicePoll{State: DeviceDenied}, nil
	}
	secret, t, err := insertToken(ctx, tx, u.ID, client, scopes, expires)
	if err != nil {
		return DevicePoll{}, err
	}
	return DevicePoll{State: DeviceIssued, Secret: secret, Token: t, User: u}, nil
}
