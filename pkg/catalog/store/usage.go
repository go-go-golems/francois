package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

// ErrAllowance means the caller's included credits are exhausted.
var ErrAllowance = errors.New("allowance exhausted")

// ErrDaily means the daily admission safeguard was reached.
var ErrDaily = errors.New("daily limit reached")

// Period is the calendar-UTC month used for included allowance.
func Period(t time.Time) string { return t.UTC().Format("2006-01") }

// PeriodReset is when the current allowance period ends.
func PeriodReset(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

type Reservation struct {
	ID      string
	CallID  string
	UserID  string
	Credits int
	Period  string
}

type ReserveRequest struct {
	UserID, InstanceID, CallID, Model string
	Credits, Included, Daily          int
	SettingsRevision                  int64
	// Period overrides the monthly ledger key; demos use "demo:<instance>"
	// so their budget never touches the visitor's monthly allowance.
	Period string
}

// Reserve atomically reserves credits for one call. Concurrent callers for
// the same user serialize on the period row, so the last credits are spent
// at most once.
func (s *Store) Reserve(ctx context.Context, r ReserveRequest) (Reservation, error) {
	period := Period(time.Now())
	if r.Period != "" {
		period = r.Period
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Reservation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var included, adjustments, reserved, used int
	err = tx.QueryRow(ctx, `INSERT INTO model_allowance_periods(user_id,period,included) VALUES($1,$2,$3)
ON CONFLICT(user_id,period) DO UPDATE SET included=EXCLUDED.included
RETURNING included,adjustments,reserved,used`, r.UserID, period, r.Included).Scan(&included, &adjustments, &reserved, &used)
	if err != nil {
		return Reservation{}, err
	}
	// The row is now locked by this transaction until commit.
	if used+reserved+r.Credits > included+adjustments {
		return Reservation{}, ErrAllowance
	}
	if r.Daily > 0 {
		var today int
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(credits),0) FROM model_calls WHERE user_id=$1 AND created_at >= date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AND state<>'released'`, r.UserID).Scan(&today); err != nil {
			return Reservation{}, err
		}
		if today+r.Credits > r.Daily {
			return Reservation{}, ErrDaily
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE model_allowance_periods SET reserved=reserved+$3 WHERE user_id=$1 AND period=$2`, r.UserID, period, r.Credits); err != nil {
		return Reservation{}, err
	}
	res := Reservation{ID: NewID("mc_"), CallID: r.CallID, UserID: r.UserID, Credits: r.Credits, Period: period}
	if _, err = tx.Exec(ctx, `INSERT INTO model_calls(id,call_id,user_id,instance_id,period,settings_revision,model,credits,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'reserved')`,
		res.ID, r.CallID, r.UserID, r.InstanceID, period, r.SettingsRevision, r.Model, r.Credits); err != nil {
		return Reservation{}, err
	}
	return res, tx.Commit(ctx)
}

// StartCall records that a provider attempt is about to start; from here the
// call is charged even if it is cancelled.
func (s *Store) StartCall(ctx context.Context, res Reservation) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE model_calls SET state='started',started_at=now() WHERE id=$1 AND state='reserved'`, res.ID)
	if err == nil && tag.RowsAffected() != 1 {
		return errors.Wrap(ErrConflict, "call is not reserved")
	}
	return err
}

type CallUsage struct {
	Input, Output, CacheRead, CacheWrite int64
	Cost                                 float64
	Rounds                               int
	ModelUsed                            string
}

// SettleCall finishes a call exactly once. A call that never started releases
// its reservation; a started call converts its reservation into usage.
func (s *Store) SettleCall(ctx context.Context, res Reservation, ok bool, code string, u CallUsage) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM model_calls WHERE id=$1 FOR UPDATE`, res.ID).Scan(&state); err != nil {
		return err
	}
	switch state {
	case "reserved":
		if _, err = tx.Exec(ctx, `UPDATE model_calls SET state='released',error_code=NULLIF($2,''),settled_at=now() WHERE id=$1`, res.ID, code); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE model_allowance_periods SET reserved=reserved-$3 WHERE user_id=$1 AND period=$2`, res.UserID, res.Period, res.Credits); err != nil {
			return err
		}
	case "started":
		final := "completed"
		if !ok {
			final = "failed"
		}
		if _, err = tx.Exec(ctx, `UPDATE model_calls SET state=$2,error_code=NULLIF($3,''),settled_at=now(),input_tokens=$4,output_tokens=$5,cache_read_tokens=$6,cache_write_tokens=$7,cost_total=$8,rounds=$9,model_used=$10 WHERE id=$1`,
			res.ID, final, code, u.Input, u.Output, u.CacheRead, u.CacheWrite, u.Cost, u.Rounds, u.ModelUsed); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE model_allowance_periods SET reserved=reserved-$3,used=used+$3 WHERE user_id=$1 AND period=$2`, res.UserID, res.Period, res.Credits); err != nil {
			return err
		}
	default:
		return nil // already settled
	}
	return tx.Commit(ctx)
}

// RecoverCalls settles calls left open by a crashed process: unstarted ones
// are released, started ones are charged and flagged for operator review.
func (s *Store) RecoverCalls(ctx context.Context) (int64, int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `WITH r AS (UPDATE model_calls SET state='released',error_code='process_restart',settled_at=now() WHERE state='reserved' RETURNING user_id,period,credits)
UPDATE model_allowance_periods p SET reserved=p.reserved-x.c FROM (SELECT user_id,period,sum(credits) c FROM r GROUP BY 1,2) x WHERE p.user_id=x.user_id AND p.period=x.period`)
	if err != nil {
		return 0, 0, err
	}
	released := tag.RowsAffected()
	tag, err = tx.Exec(ctx, `WITH r AS (UPDATE model_calls SET state='review',error_code='process_restart',settled_at=now() WHERE state='started' RETURNING user_id,period,credits)
UPDATE model_allowance_periods p SET reserved=p.reserved-x.c,used=p.used+x.c FROM (SELECT user_id,period,sum(credits) c FROM r GROUP BY 1,2) x WHERE p.user_id=x.user_id AND p.period=x.period`)
	if err != nil {
		return 0, 0, err
	}
	return released, tag.RowsAffected(), tx.Commit(ctx)
}

type Allowance struct {
	Period      string    `json:"period"`
	Included    int       `json:"included"`
	Adjustments int       `json:"adjustments"`
	Reserved    int       `json:"reserved"`
	Used        int       `json:"used"`
	Remaining   int       `json:"remaining"`
	ResetsAt    time.Time `json:"resetsAt"`
	UsedToday   int       `json:"usedToday"`
	Daily       int       `json:"daily"`
}

// AllowanceFor reports the user's current period, with the configured
// included credits when no call has created the row yet.
func (s *Store) AllowanceFor(ctx context.Context, user string, included, daily int) (Allowance, error) {
	now := time.Now()
	a := Allowance{Period: Period(now), Included: included, ResetsAt: PeriodReset(now), Daily: daily}
	err := s.Pool.QueryRow(ctx, `SELECT adjustments,reserved,used FROM model_allowance_periods WHERE user_id=$1 AND period=$2`, user, a.Period).Scan(&a.Adjustments, &a.Reserved, &a.Used)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return a, err
	}
	if err = s.Pool.QueryRow(ctx, `SELECT COALESCE(sum(credits),0) FROM model_calls WHERE user_id=$1 AND created_at >= date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AND state<>'released'`, user).Scan(&a.UsedToday); err != nil {
		return a, err
	}
	a.Remaining = a.Included + a.Adjustments - a.Used - a.Reserved
	if a.Remaining < 0 {
		a.Remaining = 0
	}
	return a, nil
}

// AdjustAllowance records an explicit audited credit adjustment.
func (s *Store) AdjustAllowance(ctx context.Context, user, actor string, credits, included int, reason string) error {
	period := Period(time.Now())
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `INSERT INTO model_allowance_periods(user_id,period,included,adjustments) VALUES($1,$2,$3,$4)
ON CONFLICT(user_id,period) DO UPDATE SET adjustments=model_allowance_periods.adjustments+EXCLUDED.adjustments`, user, period, included, credits); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO allowance_adjustments(id,user_id,period,credits,reason,actor_id) VALUES($1,$2,$3,$4,$5,$6)`, NewID("adj_"), user, period, credits, reason, actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO admin_audit(actor_id,action,subject,summary) VALUES($1,'usage.adjust',$2,jsonb_build_object('credits',$3::int,'period',$4::text,'reason',$5::text))`, actor, user, credits, period, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type ModelCall struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	UserEmail    string     `json:"userEmail"`
	InstanceID   string     `json:"instanceId"`
	Model        string     `json:"model"`
	ModelUsed    string     `json:"modelUsed"`
	Credits      int        `json:"credits"`
	State        string     `json:"state"`
	InputTokens  int64      `json:"inputTokens"`
	OutputTokens int64      `json:"outputTokens"`
	Cost         float64    `json:"cost"`
	Rounds       int        `json:"rounds"`
	ErrorCode    *string    `json:"errorCode,omitempty"`
	Revision     int64      `json:"settingsRevision"`
	CreatedAt    time.Time  `json:"createdAt"`
	SettledAt    *time.Time `json:"settledAt,omitempty"`
}

func (s *Store) ModelCalls(ctx context.Context, user string, limit int) ([]ModelCall, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `SELECT c.id,c.user_id,u.email,c.instance_id,c.model,c.model_used,c.credits,c.state,c.input_tokens,c.output_tokens,c.cost_total,c.rounds,c.error_code,c.settings_revision,c.created_at,c.settled_at
FROM model_calls c JOIN users u ON u.id=c.user_id WHERE $1='' OR c.user_id=$1 ORDER BY c.created_at DESC LIMIT $2`, user, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ModelCall, error) {
		var c ModelCall
		err := r.Scan(&c.ID, &c.UserID, &c.UserEmail, &c.InstanceID, &c.Model, &c.ModelUsed, &c.Credits, &c.State, &c.InputTokens, &c.OutputTokens, &c.Cost, &c.Rounds, &c.ErrorCode, &c.Revision, &c.CreatedAt, &c.SettledAt)
		return c, err
	})
}

type UsageSummary struct {
	Period       string  `json:"period"`
	Calls        int     `json:"calls"`
	Credits      int     `json:"credits"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	Cost         float64 `json:"cost"`
	Failed       int     `json:"failed"`
	Review       int     `json:"review"`
	ActiveUsers  int     `json:"activeUsers"`
}

func (s *Store) UsageSummary(ctx context.Context) (UsageSummary, error) {
	u := UsageSummary{Period: Period(time.Now())}
	err := s.Pool.QueryRow(ctx, `SELECT count(*),COALESCE(sum(credits) FILTER (WHERE state IN ('completed','failed','review')),0),COALESCE(sum(input_tokens),0),COALESCE(sum(output_tokens),0),COALESCE(sum(cost_total),0),
 count(*) FILTER (WHERE state='failed'),count(*) FILTER (WHERE state='review'),count(DISTINCT user_id) FROM model_calls WHERE period=$1`, u.Period).Scan(
		&u.Calls, &u.Credits, &u.InputTokens, &u.OutputTokens, &u.Cost, &u.Failed, &u.Review, &u.ActiveUsers)
	return u, err
}

// DemoCreditsToday sums the credits charged to demo calls since midnight UTC,
// across all visitors: the shared daily demo budget.
func (s *Store) DemoCreditsToday(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(sum(credits),0) FROM model_calls WHERE period LIKE 'demo:%' AND state<>'released'
 AND created_at >= date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'`).Scan(&n)
	return n, err
}
