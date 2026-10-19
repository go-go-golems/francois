package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"time"
)

var ErrAllowance = errors.New("daily generated-token ceiling reached")
var ErrSharedBudget = errors.New("shared daily demo token ceiling reached")

func Period(t time.Time) string { return t.UTC().Format("2006-01-02") }
func PeriodReset(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
}

type Reservation struct {
	ID, CallID, UserID, Period string
	Tokens                     int
}
type ReserveRequest struct {
	UserID, InstanceID, ArtifactID, CallID, Model string
	Tokens, Limit                                 int
	SettingsRevision                              int64
	Period                                        string
	Shared                                        bool
	SharedLimit                                   int
}

// Reserve locks the daily ledger before selecting a run budget. A small
// remaining balance admits a smaller run instead of requiring a full run cap.
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
	if r.Shared {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('demo-budget'))`); err != nil {
			return Reservation{}, err
		}
		var total int
		if err = tx.QueryRow(ctx, demoTokensToday).Scan(&total); err != nil {
			return Reservation{}, err
		}
		r.Tokens = min(r.Tokens, r.SharedLimit-total)
		if r.Tokens <= 0 {
			return Reservation{}, ErrSharedBudget
		}
	}
	var used, reserved int
	err = tx.QueryRow(ctx, `INSERT INTO daily_token_usage(user_id,period) VALUES($1,$2)
 ON CONFLICT(user_id,period) DO UPDATE SET period=EXCLUDED.period RETURNING used,reserved`, r.UserID, period).Scan(&used, &reserved)
	if err != nil {
		return Reservation{}, err
	}
	limit := r.Limit
	if !r.Shared {
		var override int
		err = tx.QueryRow(ctx, `SELECT daily_tokens FROM token_ceiling_overrides WHERE user_id=$1`, r.UserID).Scan(&override)
		if err == nil {
			limit = override
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return Reservation{}, err
		}
	}
	tokens := min(r.Tokens, limit-used-reserved)
	if tokens <= 0 {
		return Reservation{}, ErrAllowance
	}
	if _, err = tx.Exec(ctx, `UPDATE daily_token_usage SET reserved=reserved+$3 WHERE user_id=$1 AND period=$2`, r.UserID, period, tokens); err != nil {
		return Reservation{}, err
	}
	res := Reservation{ID: NewID("mc_"), CallID: r.CallID, UserID: r.UserID, Period: period, Tokens: tokens}
	_, err = tx.Exec(ctx, `INSERT INTO model_calls(id,call_id,user_id,instance_id,period,settings_revision,model,credits,state,artifact_id,reserved_tokens)
 VALUES($1,$2,$3,$4,$5,$6,$7,0,'reserved',$8,$9)`, res.ID, r.CallID, r.UserID, r.InstanceID, period, r.SettingsRevision, r.Model, r.ArtifactID, tokens)
	if err != nil {
		return Reservation{}, err
	}
	return res, tx.Commit(ctx)
}

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

// Unknown failed/cancelled runs retain their entire reservation. Successful
// provider reports settle measured output, never translating old credits.
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
	if state != "reserved" && state != "started" {
		return nil
	}
	charged, final := 0, "released"
	if state == "started" {
		charged = res.Tokens
		final = "review"
		if ok && u.Rounds > 0 && u.Output >= 0 && u.Output <= int64(res.Tokens) {
			charged = int(u.Output)
			final = "completed"
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE model_calls SET state=$2,error_code=NULLIF($3,''),settled_at=now(),input_tokens=$4,output_tokens=$5,cache_read_tokens=$6,cache_write_tokens=$7,cost_total=$8,rounds=$9,model_used=$10,charged_tokens=$11 WHERE id=$1`, res.ID, final, code, u.Input, u.Output, u.CacheRead, u.CacheWrite, u.Cost, u.Rounds, u.ModelUsed, charged); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE daily_token_usage SET reserved=reserved-$3,used=used+$4 WHERE user_id=$1 AND period=$2`, res.UserID, res.Period, res.Tokens, charged); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) RecoverCalls(ctx context.Context) (int64, int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `WITH r AS (UPDATE model_calls SET state='released',error_code='process_restart',settled_at=now() WHERE state='reserved' AND reserved_tokens>0 RETURNING user_id,period,reserved_tokens)
 UPDATE daily_token_usage p SET reserved=p.reserved-x.c FROM(SELECT user_id,period,sum(reserved_tokens)c FROM r GROUP BY 1,2)x WHERE p.user_id=x.user_id AND p.period=x.period`)
	if err != nil {
		return 0, 0, err
	}
	released := tag.RowsAffected()
	tag, err = tx.Exec(ctx, `WITH r AS (UPDATE model_calls SET state='review',charged_tokens=reserved_tokens,error_code='process_restart',settled_at=now() WHERE state='started' AND reserved_tokens>0 RETURNING user_id,period,reserved_tokens)
 UPDATE daily_token_usage p SET reserved=p.reserved-x.c,used=p.used+x.c FROM(SELECT user_id,period,sum(reserved_tokens)c FROM r GROUP BY 1,2)x WHERE p.user_id=x.user_id AND p.period=x.period`)
	if err != nil {
		return 0, 0, err
	}
	return released, tag.RowsAffected(), tx.Commit(ctx)
}

type Allowance struct {
	Period    string    `json:"period"`
	Default   int       `json:"default"`
	Limit     int       `json:"limit"`
	Override  *int      `json:"override"`
	Used      int       `json:"used"`
	Reserved  int       `json:"reserved"`
	Remaining int       `json:"remaining"`
	ResetsAt  time.Time `json:"resetsAt"`
}

func (s *Store) AllowanceFor(ctx context.Context, user string, limit int) (Allowance, error) {
	now := time.Now()
	a := Allowance{Period: Period(now), Default: limit, Limit: limit, ResetsAt: PeriodReset(now)}
	err := s.Pool.QueryRow(ctx, `SELECT daily_tokens FROM token_ceiling_overrides WHERE user_id=$1`, user).Scan(&a.Override)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return a, err
	}
	if a.Override != nil {
		a.Limit = *a.Override
	}
	err = s.Pool.QueryRow(ctx, `SELECT used,reserved FROM daily_token_usage WHERE user_id=$1 AND period=$2`, user, a.Period).Scan(&a.Used, &a.Reserved)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return a, err
	}
	a.Remaining = max(0, a.Limit-a.Used-a.Reserved)
	return a, nil
}
func (s *Store) SetTokenCeiling(ctx context.Context, user, actor string, limit *int, reason string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if limit == nil {
		_, err = tx.Exec(ctx, `DELETE FROM token_ceiling_overrides WHERE user_id=$1`, user)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO token_ceiling_overrides(user_id,daily_tokens) VALUES($1,$2) ON CONFLICT(user_id)DO UPDATE SET daily_tokens=EXCLUDED.daily_tokens`, user, *limit)
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO admin_audit(actor_id,token_id,action,subject,summary)VALUES($1,$2,'usage.ceiling',$3,jsonb_build_object('dailyTokens',$4::int,'reason',$5::text))`, actor, auditToken(ctx), user, limit, reason)
	if err != nil {
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

// Shared daily usage includes reservations and conservative unknown charges.
const demoTokensToday = `SELECT COALESCE(sum(CASE WHEN state IN ('reserved','started') THEN reserved_tokens ELSE charged_tokens END),0) FROM model_calls WHERE period LIKE 'demo:%' AND state<>'released' AND created_at>=date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'`
