package store

import (
	"context"
	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"time"
)

// YearCheckouts are owner-bound attempts needing authoritative payment refresh.
func (s *Store) YearCheckouts(ctx context.Context, user string) ([]CheckoutAttempt, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM checkout_attempts WHERE user_id=$1 AND price=$2 AND session_id IS NOT NULL AND state IN ('open','completed') AND (support_until IS NULL OR support_until>now())`, user, billing.YearFundingKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CheckoutAttempt
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// YearSupport returns the most recent, still-paid-through one-time contribution.
func (s *Store) YearSupport(ctx context.Context, user string, now time.Time) (billing.YearSupport, error) {
	var out billing.YearSupport
	err := s.Pool.QueryRow(ctx, `SELECT id,support_paid_at,support_until FROM checkout_attempts WHERE user_id=$1 AND price=$2 AND support_until>$3 ORDER BY support_until DESC LIMIT 1`, user, billing.YearFundingKey, now).Scan(&out.AttemptID, &out.PaidAt, &out.Until)
	return out, notFound(err)
}
