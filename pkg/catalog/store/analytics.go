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
type Analytics struct {
	From    time.Time                 `json:"from"`
	To      time.Time                 `json:"to"`
	Site    string                    `json:"site"`
	Traffic TrafficTotals             `json:"traffic"`
	Groups  map[string][]TrafficGroup `json:"groups"`
	Models  map[string][]ModelGroup   `json:"models"`
}

// Analytics uses a half-open UTC range. Traffic can be site-filtered; model
// calls are catalog-wide because the billing ledger has no portal-site field.
func (s *Store) Analytics(ctx context.Context, from, to time.Time, site string) (Analytics, error) {
	a := Analytics{From: from, To: to, Site: site, Groups: map[string][]TrafficGroup{}, Models: map[string][]ModelGroup{}}
	const filter = ` FROM traffic_views WHERE created_at >= $1 AND created_at < $2 AND ($3='' OR site=$3)`
	err := s.Pool.QueryRow(ctx, `SELECT count(*),count(DISTINCT (site,visitor)),count(DISTINCT (site,session)),count(DISTINCT NULLIF(user_id,'')),(SELECT count(*) FROM (SELECT site,session`+filter+` GROUP BY site,session HAVING count(*)=1) b)`+filter, from, to, site).Scan(&a.Traffic.Views, &a.Traffic.Visitors, &a.Traffic.Sessions, &a.Traffic.Users, &a.Traffic.Bounces)
	if err != nil {
		return a, err
	}
	// Expressions are fixed server constants, never request-provided SQL.
	for _, g := range []struct{ name, expr string }{
		{"daily", `to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD')`},
		{"pages", "path"}, {"referrers", "referrer"}, {"sources", "source"}, {"mediums", "medium"}, {"campaigns", "campaign"}, {"devices", "device"}, {"browsers", "browser"}, {"users", "user_id"},
	} {
		rows, err := s.Pool.Query(ctx, `SELECT `+g.expr+`,count(*),count(DISTINCT (site,visitor))`+filter+` GROUP BY 1 ORDER BY count(*) DESC,1 LIMIT 100`, from, to, site)
		if err != nil {
			return a, err
		}
		groups, err := pgx.CollectRows(rows, pgx.RowToStructByPos[TrafficGroup])
		if err != nil {
			return a, err
		}
		a.Groups[g.name] = groups
	}
	for _, g := range []struct{ name, expr string }{{"daily", `to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD')`}, {"models", "COALESCE(NULLIF(model_used,''),model)"}, {"users", "user_id"}, {"instances", "instance_id"}} {
		rows, err := s.Pool.Query(ctx, `SELECT `+g.expr+`,count(*),count(*) FILTER(WHERE state='failed'),COALESCE(sum(credits) FILTER(WHERE state IN ('completed','failed','review')),0),COALESCE(sum(input_tokens),0),COALESCE(sum(output_tokens),0),COALESCE(sum(cache_read_tokens),0),COALESCE(sum(cache_write_tokens),0),COALESCE(sum(cost_total),0) FROM model_calls WHERE created_at >= $1 AND created_at < $2 GROUP BY 1 ORDER BY count(*) DESC,1 LIMIT 100`, from, to)
		if err != nil {
			return a, err
		}
		groups, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ModelGroup])
		if err != nil {
			return a, err
		}
		a.Models[g.name] = groups
	}
	return a, nil
}

func (s *Store) PruneTraffic(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM traffic_views WHERE created_at < now()-interval '90 days'`)
	return err
}
