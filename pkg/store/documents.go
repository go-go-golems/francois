package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Document struct {
	ID      string         `json:"id"`
	Path    string         `json:"path"`
	Exists  bool           `json:"exists"`
	Body    map[string]any `json:"body"`
	Version int64          `json:"version"`
}
type Filter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}
type Query struct {
	Owner     string   `json:"-"`
	Path      string   `json:"path"`
	Filters   []Filter `json:"filters"`
	Order     string   `json:"order"`
	Direction string   `json:"direction"`
	Limit     int      `json:"limit"`
}

var segment = regexp.MustCompile(`^[A-Za-z0-9_.~:@+-]+$`)

func ValidatePath(path string, document bool) error {
	parts := strings.Split(path, "/")
	if len(path) > 1000 || len(parts) > 16 || (len(parts)%2 == 0) != document {
		return errors.New("Invalid document or collection path")
	}
	for _, p := range parts {
		if !segment.MatchString(p) || len(p) > 200 {
			return errors.New("Invalid path segment")
		}
	}
	return nil
}
func (s *Store) Document(ctx context.Context, artifact, path string) (Document, error) {
	d := Document{ID: path[strings.LastIndex(path, "/")+1:], Path: path}
	var body string
	err := s.DB.QueryRowContext(ctx, `SELECT body,version FROM docs WHERE artifact=? AND path=?`, artifact, path).Scan(&body, &d.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	d.Exists = true
	err = json.Unmarshal([]byte(body), &d.Body)
	return d, err
}
func Merge(dst, patch map[string]any) map[string]any {
	for k, v := range patch {
		p, ok := v.(map[string]any)
		old, oldOK := dst[k].(map[string]any)
		if ok && oldOK {
			dst[k] = Merge(old, p)
		} else {
			dst[k] = v
		}
	}
	return dst
}
func (s *Store) Write(ctx context.Context, artifact, path, operation string, body map[string]any) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if operation == "delete" {
		_, err = tx.ExecContext(ctx, `DELETE FROM docs WHERE artifact=? AND path=?`, artifact, path)
	} else {
		if operation == "update" {
			var old string
			if err = tx.QueryRowContext(ctx, `SELECT body FROM docs WHERE artifact=? AND path=?`, artifact, path).Scan(&old); err != nil {
				return errors.New("Cannot update a missing document")
			}
			var data map[string]any
			if err = json.Unmarshal([]byte(old), &data); err != nil {
				return err
			}
			body = Merge(data, body)
		}
		raw, e := json.Marshal(body)
		if e != nil {
			return e
		}
		if len(raw) > 262144 {
			return errors.New("Document exceeds 256 KiB")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO docs VALUES(?,?,?,1) ON CONFLICT(artifact,path) DO UPDATE SET body=excluded.body,version=docs.version+1`, artifact, path, string(raw))
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Query(ctx context.Context, artifact string, q Query) ([]Document, error) {
	if err := ValidatePath(q.Path, false); err != nil {
		return nil, err
	}
	if len(q.Filters) > 10 || q.Limit < 0 || q.Limit > 1000 {
		return nil, errors.New("Invalid query limits")
	}
	if q.Direction != "" && q.Direction != "asc" && q.Direction != "desc" {
		return nil, errors.New("Invalid ordering")
	}
	for _, f := range q.Filters {
		switch f.Op {
		case "==", "!=", "<", "<=", ">", ">=", "in", "not-in", "array-contains":
		default:
			return nil, errors.New("Invalid filter operator")
		}
		if f.Field == "" || strings.Contains(f.Field, ".") {
			return nil, errors.New("Filters use top-level fields")
		}
		if f.Op == "in" || f.Op == "not-in" {
			a, ok := f.Value.([]any)
			if !ok || len(a) == 0 || len(a) > 30 {
				return nil, errors.New("in filters need 1–30 values")
			}
		}
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT path,body,version FROM docs WHERE artifact=? ORDER BY path`, artifact)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	docs := []Document{}
	count := 0
	for rows.Next() {
		var d Document
		var body string
		if err = rows.Scan(&d.Path, &body, &d.Version); err != nil {
			return nil, err
		}
		if q.Owner != "" && strings.HasPrefix(d.Path, "data/users/") {
			parts := strings.Split(d.Path, "/")
			if len(parts) < 3 || parts[2] != q.Owner {
				continue
			}
		}
		if !strings.HasPrefix(d.Path, q.Path+"/") {
			continue
		}
		rest := strings.TrimPrefix(d.Path, q.Path+"/")
		if strings.Contains(rest, "/") {
			continue
		}
		count++
		if count > 5000 {
			return nil, errors.New("Query exceeds 5000 document scan limit")
		}
		if err = json.Unmarshal([]byte(body), &d.Body); err != nil {
			return nil, err
		}
		d.ID = rest
		d.Exists = true
		match := true
		for _, f := range q.Filters {
			v, ok := d.Body[f.Field]
			if !ok || !matches(v, f) {
				match = false
				break
			}
		}
		if match {
			docs = append(docs, d)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(docs, func(i, j int) bool {
		a, b := docs[i], docs[j]
		if q.Order != "" {
			av, ao := a.Body[q.Order]
			bv, bo := b.Body[q.Order]
			if ao != bo {
				return ao
			}
			if ao {
				c := compare(av, bv)
				if c != 0 {
					if q.Direction == "desc" {
						return c > 0
					}
					return c < 0
				}
			}
		}
		return a.ID < b.ID
	})
	if q.Limit > 0 && len(docs) > q.Limit {
		docs = docs[:q.Limit]
	}
	return docs, nil
}
func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func compare(a, b any) int {
	if x, ok := a.(float64); ok {
		if y, ok := b.(float64); ok {
			if x < y {
				return -1
			}
			if x > y {
				return 1
			}
			return 0
		}
	}
	if x, ok := a.(string); ok {
		if y, ok := b.(string); ok {
			return strings.Compare(x, y)
		}
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return strings.Compare(string(x), string(y))
}
func matches(v any, f Filter) bool {
	switch f.Op {
	case "==":
		return equal(v, f.Value)
	case "!=":
		return !equal(v, f.Value)
	case "<":
		return compare(v, f.Value) < 0
	case "<=":
		return compare(v, f.Value) <= 0
	case ">":
		return compare(v, f.Value) > 0
	case ">=":
		return compare(v, f.Value) >= 0
	case "in", "not-in":
		a, _ := f.Value.([]any)
		found := false
		for _, item := range a {
			if equal(v, item) {
				found = true
			}
		}
		if f.Op == "not-in" {
			return !found
		}
		return found
	case "array-contains":
		a, _ := v.([]any)
		for _, item := range a {
			if equal(item, f.Value) {
				return true
			}
		}
	}
	return false
}
func (s *Store) Acquire(ctx context.Context, artifact, path, holder string, ttl int64, data any) (bool, error) {
	if holder == "" || ttl < 1000 || ttl > 600000 {
		return false, errors.New("Invalid lease holder or TTL")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var existing string
	var expires int64
	now := time.Now().UnixMilli()
	err = tx.QueryRowContext(ctx, `SELECT holder,expires FROM leases WHERE artifact=? AND path=?`, artifact, path).Scan(&existing, &expires)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err == nil && expires > now && existing != holder {
		return false, nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO leases VALUES(?,?,?,?,?) ON CONFLICT(artifact,path) DO UPDATE SET holder=excluded.holder,expires=excluded.expires,data=excluded.data`, artifact, path, holder, now+ttl, string(raw))
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
