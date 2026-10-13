package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

type SettingsRevision struct {
	Revision    int64             `json:"revision"`
	Document    settings.Document `json:"document"`
	ActorID     *string           `json:"actorId"`
	ActivatedAt time.Time         `json:"activatedAt"`
}

// ActiveSettings returns the newest revision, or revision 0 with defaults.
func (s *Store) ActiveSettings(ctx context.Context) (SettingsRevision, error) {
	var r SettingsRevision
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT revision,document,actor_id,activated_at FROM platform_settings ORDER BY revision DESC LIMIT 1`).Scan(&r.Revision, &raw, &r.ActorID, &r.ActivatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SettingsRevision{Document: settings.Defaults()}, nil
	}
	if err != nil {
		return r, err
	}
	r.Document, err = settings.Parse(raw)
	return r, err
}

// SaveSettings validates and activates a new revision with an audit record.
func (s *Store) SaveSettings(ctx context.Context, actor string, d settings.Document) (SettingsRevision, error) {
	if err := d.Validate(); err != nil {
		return SettingsRevision{}, err
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return SettingsRevision{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return SettingsRevision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r := SettingsRevision{Document: d}
	var a any
	if actor != "" {
		a = actor
		r.ActorID = &actor
	}
	if err = tx.QueryRow(ctx, `INSERT INTO platform_settings(document,actor_id) VALUES($1,$2) RETURNING revision,activated_at`, raw, a).Scan(&r.Revision, &r.ActivatedAt); err != nil {
		return r, err
	}
	summary, _ := json.Marshal(map[string]any{"revision": r.Revision})
	if _, err = tx.Exec(ctx, `INSERT INTO admin_audit(actor_id,action,subject,summary) VALUES($1,'settings.activate','settings',$2)`, a, summary); err != nil {
		return r, err
	}
	return r, tx.Commit(ctx)
}

func (s *Store) SettingsHistory(ctx context.Context, limit int) ([]SettingsRevision, error) {
	rows, err := s.Pool.Query(ctx, `SELECT revision,document,actor_id,activated_at FROM platform_settings ORDER BY revision DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SettingsRevision, error) {
		var r SettingsRevision
		var raw []byte
		if err := row.Scan(&r.Revision, &raw, &r.ActorID, &r.ActivatedAt); err != nil {
			return r, err
		}
		return r, json.Unmarshal(raw, &r.Document)
	})
}
