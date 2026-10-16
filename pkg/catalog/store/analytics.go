package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type TrafficView struct {
	Site, Path, Visitor, Session, UserID, Referrer, Source, Medium, Campaign, Device, Browser string
}

func (s *Store) RecordView(ctx context.Context, v TrafficView) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO traffic_views(site,path,visitor,session,user_id,referrer,source,medium,campaign,device,browser) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.Site, v.Path, v.Visitor, v.Session, v.UserID, v.Referrer, v.Source, v.Medium, v.Campaign, v.Device, v.Browser)
	return err
}

type TrafficTotals struct {
	Views    int64 `json:"views"`
	Visitors int64 `json:"visitors"`
	Sessions int64 `json:"sessions"`
	Users    int64 `json:"users"`
	Bounces  int64 `json:"bounces"`
}
type TrafficGroup struct {
	Key      string `json:"key"`
	Views    int64  `json:"views"`
	Visitors int64  `json:"visitors"`
}
type ModelGroup struct {
	Label      string  `json:"label,omitempty"`
	Key        string  `json:"key"`
	Calls      int64   `json:"calls"`
	Failed     int64   `json:"failed"`
	Credits    int64   `json:"credits"`
	Input      int64   `json:"inputTokens"`
	Output     int64   `json:"outputTokens"`
	CacheRead  int64   `json:"cacheReadTokens"`
	CacheWrite int64   `json:"cacheWriteTokens"`
	Cost       float64 `json:"cost"`
}

// TrafficFilter applies only to portal traffic. A nil referrer means all;
// an explicitly empty value selects direct/unspecified views.
type TrafficFilter struct {
	Site     string  `json:"site"`
	Path     string  `json:"path"`
	Referrer *string `json:"referrer"`
}

type TrafficPoint struct {
	Day   string `json:"day"`
	Key   string `json:"key"`
	Other bool   `json:"other"`
	Views int64  `json:"views"`
}

type Analytics struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	TrafficFilter
	Series  map[string][]TrafficPoint `json:"series"`
	Traffic TrafficTotals             `json:"traffic"`
	Groups  map[string][]TrafficGroup `json:"groups"`
	Models  map[string][]ModelGroup   `json:"models"`
}

// Analytics uses a half-open UTC range. Traffic can be site-filtered; model
// calls are catalog-wide because the billing ledger has no portal-site field.
func (s *Store) Analytics(ctx context.Context, from, to time.Time, f TrafficFilter) (Analytics, error) {
	a := Analytics{From: from, To: to, TrafficFilter: f, Series: map[string][]TrafficPoint{}, Groups: map[string][]TrafficGroup{}, Models: map[string][]ModelGroup{}}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return a, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	args := []any{from, to, f.Site, f.Path, f.Referrer}
	const filter = ` FROM traffic_views WHERE created_at >= $1 AND created_at < $2 AND ($3='' OR site=$3) AND ($4='' OR path=$4) AND ($5::text IS NULL OR referrer=$5)`
	err = tx.QueryRow(ctx, `SELECT count(*),count(DISTINCT (site,visitor)),count(DISTINCT (site,session)),count(DISTINCT NULLIF(user_id,'')),(SELECT count(*) FROM (SELECT site,session`+filter+` GROUP BY site,session HAVING count(*)=1) b)`+filter, args...).Scan(&a.Traffic.Views, &a.Traffic.Visitors, &a.Traffic.Sessions, &a.Traffic.Users, &a.Traffic.Bounces)
	if err != nil {
		return a, err
	}
	// Expressions are fixed server constants, never request-provided SQL.
	for _, g := range []struct{ name, expr string }{
		{"daily", `to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD')`},
		{"pages", "path"}, {"referrers", "referrer"}, {"sources", "source"}, {"mediums", "medium"}, {"campaigns", "campaign"}, {"devices", "device"}, {"browsers", "browser"}, {"users", "user_id"},
	} {
		rows, err := tx.Query(ctx, `SELECT `+g.expr+`,count(*),count(DISTINCT (site,visitor))`+filter+` GROUP BY 1 ORDER BY count(*) DESC,1 LIMIT 100`, args...)
		if err != nil {
			return a, err
		}
		groups, err := pgx.CollectRows(rows, pgx.RowToStructByPos[TrafficGroup])
		if err != nil {
			return a, err
		}
		a.Groups[g.name] = groups
	}
	// Compute complete daily stacks independently of the top-100 browsing tables.
	for _, g := range []struct{ name, expr string }{{"pages", "path"}, {"referrers", "referrer"}} {
		rows, err := tx.Query(ctx, `WITH filtered AS (SELECT created_at,`+g.expr+` AS category`+filter+`),
   top_keys AS (SELECT category FROM filtered GROUP BY category ORDER BY count(*) DESC,category LIMIT 3)
   SELECT to_char(f.created_at AT TIME ZONE 'UTC','YYYY-MM-DD'),COALESCE(k.category,''),k.category IS NULL,count(*)
   FROM filtered f LEFT JOIN top_keys k ON k.category=f.category GROUP BY 1,2,3 ORDER BY 1,3,2`, args...)
		if err != nil {
			return a, err
		}
		points, err := pgx.CollectRows(rows, pgx.RowToStructByPos[TrafficPoint])
		if err != nil {
			return a, err
		}
		a.Series[g.name] = points
	}
	for _, g := range []struct{ name, expr string }{{"daily", `to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD')`}, {"models", "COALESCE(NULLIF(model_used,''),model)"}, {"users", "user_id"}, {"instances", "instance_id"}, {"artifacts", "artifact_id"}} {
		rows, err := tx.Query(ctx, `SELECT ''::text,`+g.expr+`,count(*),count(*) FILTER(WHERE state='failed'),COALESCE(sum(credits) FILTER(WHERE state IN ('completed','failed','review')),0),COALESCE(sum(input_tokens),0),COALESCE(sum(output_tokens),0),COALESCE(sum(cache_read_tokens),0),COALESCE(sum(cache_write_tokens),0),COALESCE(sum(cost_total),0) FROM model_calls WHERE created_at >= $1 AND created_at < $2 GROUP BY 2 ORDER BY count(*) DESC,2 LIMIT 100`, from, to)
		if err != nil {
			return a, err
		}
		groups, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ModelGroup])
		if err != nil {
			return a, err
		}
		a.Models[g.name] = groups
	}
	// Labels are a one-to-one display lookup; never join creators/members into usage sums.
	ids := make([]string, 0, len(a.Models["artifacts"]))
	for _, g := range a.Models["artifacts"] {
		if g.Key != "" {
			ids = append(ids, g.Key)
		}
	}
	rows, err := tx.Query(ctx, `SELECT id,title FROM catalog_artifacts WHERE id=ANY($1)`, ids)
	if err != nil {
		return a, err
	}
	names, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) ([2]string, error) {
		var v [2]string
		err := r.Scan(&v[0], &v[1])
		return v, err
	})
	if err != nil {
		return a, err
	}
	labels := map[string]string{}
	for _, v := range names {
		labels[v[0]] = v[1]
	}
	for i := range a.Models["artifacts"] {
		a.Models["artifacts"][i].Label = labels[a.Models["artifacts"][i].Key]
	}
	return a, tx.Commit(ctx)
}

func (s *Store) PruneTraffic(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM traffic_views WHERE created_at < now()-interval '90 days'`)
	return err
}
