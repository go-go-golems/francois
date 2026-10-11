package host

import (
	"encoding/json"
	"github.com/pkg/errors"
	"net/http"
	"time"
)

func (h *Host) commentOperation(r *http.Request, v *View, method, thread, text string, anchor any, resolved bool) (any, error) {
	ctx := r.Context()
	if len(text) > 4096 {
		return nil, errors.New("Comment text exceeds 4 KiB")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch method {
	case "comments.create":
		if text == "" || anchor == nil {
			return nil, errors.New("Comment requires text and anchor")
		}
		raw, err := json.Marshal(anchor)
		if err != nil || len(raw) > 4096 {
			return nil, errors.New("Invalid comment anchor")
		}
		tx, err := h.Store.DB.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback() }()
		thread = id("t_")
		comment := id("m_")
		if _, err = tx.ExecContext(ctx, `INSERT INTO threads VALUES(?,?,?,0,?)`, thread, v.Artifact, string(raw), now); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO comments VALUES(?,?,?,?,?)`, comment, thread, h.Owner, text, now); err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return map[string]string{"threadId": thread, "commentId": comment}, nil
	case "comments.reply", "comments.resolve", "comments.delete":
		var count int
		if err := h.Store.DB.QueryRowContext(ctx, `SELECT count(*) FROM threads WHERE id=? AND artifact=?`, thread, v.Artifact).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, errors.New("Thread not found")
		}
		switch method {
		case "comments.reply":
			if text == "" {
				return nil, errors.New("Reply cannot be empty")
			}
			_, err := h.Store.DB.ExecContext(ctx, `INSERT INTO comments VALUES(?,?,?,?,?)`, id("m_"), thread, h.Owner, text, now)
			return map[string]bool{"ok": true}, err
		case "comments.resolve":
			_, err := h.Store.DB.ExecContext(ctx, `UPDATE threads SET resolved=? WHERE id=?`, resolved, thread)
			return map[string]bool{"ok": true}, err
		case "comments.delete":
			tx, err := h.Store.DB.BeginTx(ctx, nil)
			if err != nil {
				return nil, err
			}
			defer func() { _ = tx.Rollback() }()
			if _, err = tx.ExecContext(ctx, `DELETE FROM comments WHERE thread=?`, thread); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `DELETE FROM threads WHERE id=?`, thread); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, tx.Commit()
		}
	}
	return nil, errors.New("Unknown comment operation")
}
func (h *Host) listThreads(w http.ResponseWriter, r *http.Request, artifact string) {
	// Shell-only read API; no thread listing is exposed in window.claude.
	rows, err := h.Store.DB.QueryContext(r.Context(), `SELECT t.id,t.anchor,t.resolved,c.id,c.text,c.created_at FROM threads t JOIN comments c ON c.thread=t.id WHERE t.artifact=? ORDER BY c.created_at,c.id`, artifact)
	if err != nil {
		fail(w, 500, "unavailable", "Comments unavailable")
		return
	}
	defer func() { _ = rows.Close() }()
	out := []map[string]any{}
	for rows.Next() {
		var thread, anchor, key, text, at string
		var resolved bool
		if err = rows.Scan(&thread, &anchor, &resolved, &key, &text, &at); err != nil {
			fail(w, 500, "unavailable", "Comments unavailable")
			return
		}
		out = append(out, map[string]any{"threadId": thread, "anchor": json.RawMessage(anchor), "resolved": resolved, "id": key, "text": text, "createdAt": at})
	}
	if err = rows.Err(); err != nil {
		fail(w, 500, "unavailable", "Comments unavailable")
		return
	}
	respond(w, out)
}
