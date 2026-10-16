package catalog

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// mint creates a token through the browser-session API and returns its secret
// and ID.
func (e *env) mint(a *tactor, days int, scopes ...string) (string, string) {
	e.t.Helper()
	code, out := e.portal(a, "POST", "/api/tokens", map[string]any{"name": "t", "days": days, "scopes": scopes})
	if code != 201 {
		e.t.Fatalf("mint %v for %s: %d %v", scopes, a.user.Role, code, out)
	}
	return out["token"].(string), out["info"].(map[string]any)["id"].(string)
}

// bearer sends a request authenticated only by a personal API token.
func (e *env) bearer(secret, method, path, body string) (int, string) {
	e.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	code, b, _ := e.do(nil, "portal.test", method, path, r, map[string]string{"Authorization": "Bearer " + secret})
	return code, string(b)
}

// TestTokenScopeMatrix checks which surfaces each kind of token reaches. The
// scope is a ceiling: the owner's role still applies.
func TestTokenScopeMatrix(t *testing.T) {
	e := setup(t)
	sam := e.user("sam", "subscriber", true)
	cleo := e.user("cleo", "creator", false)
	ada := e.user("ada", "admin", false)

	samAccount, _ := e.mint(&sam, 0, "account")
	cleoStudio, _ := e.mint(&cleo, 0, "studio")
	cleoAccount, _ := e.mint(&cleo, 0, "account")
	adaRead, _ := e.mint(&ada, 30, "admin:read")
	adaWrite, _ := e.mint(&ada, 30, "admin:write")
	adaStudioWrite, _ := e.mint(&ada, 30, "studio", "admin:write")

	type call struct{ method, path, body string }
	studioGet := call{"GET", "/api/studio/artifacts", ""}
	accountGet := call{"GET", "/api/account", ""}
	adminGet := call{"GET", "/api/admin/users", ""}
	adminWrite := call{"PATCH", "/api/admin/users/" + sam.user.ID, `{"disabled":false}`}
	me := call{"GET", "/api/me", ""}
	public := call{"GET", "/api/catalog", ""}
	mint := call{"POST", "/api/tokens", `{"name":"x","scopes":["account"]}`}
	launch := call{"POST", "/api/instances/ins_missing/launch", `{}`}

	// want is "200" or an error code.
	cases := []struct {
		name   string
		secret string
		call   call
		want   string
	}{
		{"subscriber account → account", samAccount, accountGet, "200"},
		{"subscriber account → studio", samAccount, studioGet, "token_scope"},
		{"subscriber account → admin", samAccount, adminGet, "token_scope"},
		{"creator studio → studio", cleoStudio, studioGet, "200"},
		{"creator studio → account", cleoStudio, accountGet, "token_scope"},
		{"creator account → account", cleoAccount, accountGet, "200"},
		{"creator account → studio", cleoAccount, studioGet, "token_scope"},
		{"admin read → admin GET", adaRead, adminGet, "200"},
		{"admin read → admin write", adaRead, adminWrite, "token_scope"},
		{"admin read → studio", adaRead, studioGet, "token_scope"},
		{"admin write → admin GET", adaWrite, adminGet, "200"},
		{"admin write → admin write", adaWrite, adminWrite, "200"},
		{"admin studio+write → studio", adaStudioWrite, studioGet, "200"},
		{"admin studio+write → account", adaStudioWrite, accountGet, "token_scope"},
		{"any token → me", samAccount, me, "200"},
		{"any token → public route", adaWrite, public, "token_not_allowed"},
		{"any token → token management", adaWrite, mint, "token_not_allowed"},
		{"any token → launch", samAccount, launch, "token_not_allowed"},
	}
	for _, c := range cases {
		code, body := e.bearer(c.secret, c.call.method, c.call.path, c.call.body)
		if c.want == "200" {
			if code != 200 {
				t.Errorf("%s: %d %s", c.name, code, body)
			}
			continue
		}
		if !strings.Contains(body, `"code":"`+c.want+`"`) {
			t.Errorf("%s: %d %s, want %s", c.name, code, body, c.want)
		}
	}

	// /api/me reports the identity and the token's scopes.
	code, body := e.bearer(cleoStudio, "GET", "/api/me", "")
	var meOut struct {
		User  struct{ Role string }
		Token struct{ Scopes []string }
	}
	if code != 200 || json.Unmarshal([]byte(body), &meOut) != nil || meOut.User.Role != "creator" || len(meOut.Token.Scopes) != 1 || meOut.Token.Scopes[0] != "studio" {
		t.Fatalf("me %d %s", code, body)
	}
	if code, out := e.portal(&cleo, "GET", "/api/me", nil); code != 200 || out["token"] != nil {
		t.Fatalf("me with a session %d %v", code, out)
	}

	// The scope is a ceiling: a demoted administrator's admin token stops working.
	ctx := context.Background()
	if err := e.store.SetUserRole(ctx, ada.user.ID, "creator"); err != nil {
		t.Fatal(err)
	}
	if code, body := e.bearer(adaWrite, "GET", "/api/admin/users", ""); code != 403 || !strings.Contains(body, "admin_required") {
		t.Fatalf("demoted admin token %d %s", code, body)
	}

	// A disabled owner's token cannot act; reading the account stays possible,
	// as it does for the browser session.
	if err := e.store.SetUserDisabled(ctx, sam.user.ID, true); err != nil {
		t.Fatal(err)
	}
	if code, body := e.bearer(samAccount, "POST", "/api/instances", `{"releaseId":"rel_x"}`); code != 403 || !strings.Contains(body, "account_disabled") {
		t.Fatalf("disabled owner's token %d %s", code, body)
	}
	if code, body := e.bearer(samAccount, "GET", "/api/account", ""); code != 200 {
		t.Fatalf("disabled owner reading account %d %s", code, body)
	}

	// An expired token is invalid.
	if _, err := e.store.Pool.Exec(ctx, `UPDATE api_tokens SET expires_at=now()-interval '1 minute' WHERE user_id=$1`, cleo.user.ID); err != nil {
		t.Fatal(err)
	}
	if code, body := e.bearer(cleoStudio, "GET", "/api/studio/artifacts", ""); code != 401 || !strings.Contains(body, "token_invalid") {
		t.Fatalf("expired token %d %s", code, body)
	}
}

func TestTokenMintingRules(t *testing.T) {
	e := setup(t)
	sam := e.user("sam", "subscriber", true)
	cleo := e.user("cleo", "creator", false)
	ada := e.user("ada", "admin", false)

	cases := []struct {
		name   string
		who    *tactor
		body   map[string]any
		status int
		code   string
	}{
		{"subscriber may hold account", &sam, map[string]any{"name": "a", "scopes": []string{"account"}}, 201, ""},
		{"subscriber may not hold studio", &sam, map[string]any{"name": "a", "scopes": []string{"studio"}}, 400, "invalid_scopes"},
		{"creator may not hold admin:read", &cleo, map[string]any{"name": "a", "days": 30, "scopes": []string{"admin:read"}}, 400, "invalid_scopes"},
		{"unknown scope", &ada, map[string]any{"name": "a", "scopes": []string{"root"}}, 400, "invalid_scopes"},
		{"no scope", &cleo, map[string]any{"name": "a"}, 400, "invalid_scopes"},
		{"admin token without expiry", &ada, map[string]any{"name": "a", "scopes": []string{"admin:write"}}, 400, "invalid_request"},
		{"admin token over 90 days", &ada, map[string]any{"name": "a", "days": 91, "scopes": []string{"admin:read"}}, 400, "invalid_request"},
		{"admin token for 90 days", &ada, map[string]any{"name": "a", "days": 90, "scopes": []string{"admin:read", "studio"}}, 201, ""},
		{"creator studio without expiry", &cleo, map[string]any{"name": "a", "scopes": []string{"studio", "studio"}}, 201, ""},
	}
	for _, c := range cases {
		code, out := e.portal(c.who, "POST", "/api/tokens", c.body)
		if code != c.status || (c.code != "" && errCode(out) != c.code) {
			t.Errorf("%s: %d %v", c.name, code, out)
		}
		if c.name == "creator studio without expiry" {
			if scopes := out["info"].(map[string]any)["scopes"].([]any); len(scopes) != 1 {
				t.Errorf("duplicate scopes kept: %v", scopes)
			}
		}
	}

	// Creation is audited with the scopes.
	entries, err := e.store.AuditLog(context.Background(), "", 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, en := range entries {
		if en.Action == "token.create" && strings.Contains(string(en.Summary), `"admin:read"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("token.create audit entry with scopes missing")
	}
}

// TestAuditRecordsToken checks that a change made through a token names the
// token in the audit log, and a change made in the browser does not.
func TestAuditRecordsToken(t *testing.T) {
	e := setup(t)
	ada := e.user("ada", "admin", false)
	secret, tokenID := e.mint(&ada, 30, "studio")

	if code, body := e.bearer(secret, "POST", "/api/studio/artifacts", `{"id":"viatoken","title":"Via token"}`); code != 201 {
		t.Fatalf("create through token %d %s", code, body)
	}
	if code, out := e.portal(&ada, "POST", "/api/studio/artifacts", map[string]any{"id": "viasession", "title": "Via session"}); code != 201 {
		t.Fatalf("create through session %d %v", code, out)
	}
	ctx := context.Background()
	for subject, want := range map[string]string{"viatoken": tokenID, "viasession": ""} {
		entries, err := e.store.AuditLog(ctx, subject, 10)
		if err != nil || len(entries) == 0 {
			t.Fatalf("audit for %s: %v %v", subject, entries, err)
		}
		got := ""
		if entries[0].TokenID != nil {
			got = *entries[0].TokenID
		}
		if got != want {
			t.Errorf("audit %s for %s: token %q, want %q", entries[0].Action, subject, got, want)
		}
	}
}

func TestAdminTokenOversight(t *testing.T) {
	e := setup(t)
	ada := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	first, firstID := e.mint(&cleo, 0, "studio")
	second, _ := e.mint(&cleo, 0, "account")

	code, list, _ := e.do(&ada, "portal.test", "GET", "/api/admin/users/"+cleo.user.ID+"/tokens", nil, nil)
	if code != 200 || !strings.Contains(string(list), firstID) || strings.Contains(string(list), first) {
		t.Fatalf("admin token list %d %s", code, list)
	}
	if code, _ := e.portal(&ada, "GET", "/api/admin/users/usr_missing/tokens", nil); code != 404 {
		t.Fatalf("tokens of an unknown user: %d", code)
	}
	if code, _ := e.portal(&cleo, "GET", "/api/admin/users/"+cleo.user.ID+"/tokens", nil); code != 403 {
		t.Fatalf("creator listing tokens through the admin API: %d", code)
	}

	// Revoke one leaked token.
	if code, out := e.portal(&ada, "DELETE", "/api/admin/tokens/"+firstID, nil); code != 200 {
		t.Fatalf("admin revoke %d %v", code, out)
	}
	if code, body := e.bearer(first, "GET", "/api/studio/artifacts", ""); code != 401 || !strings.Contains(body, "token_invalid") {
		t.Fatalf("revoked token %d %s", code, body)
	}
	if code, _ := e.portal(&ada, "DELETE", "/api/admin/tokens/"+firstID, nil); code != 404 {
		t.Fatal("revoking a revoked token")
	}
	entries, _ := e.store.AuditLog(context.Background(), cleo.user.ID, 10)
	if len(entries) == 0 || entries[0].Action != "token.revoke" {
		t.Fatalf("token.revoke not audited against the owner: %+v", entries)
	}

	// Revoking sessions leaves tokens alone unless asked.
	if code, out := e.portal(&ada, "POST", "/api/admin/users/"+cleo.user.ID+"/revoke-sessions", nil); code != 200 || out["tokens"] != float64(0) {
		t.Fatalf("revoke sessions %d %v", code, out)
	}
	if code, _ := e.bearer(second, "GET", "/api/me", ""); code != 200 {
		t.Fatal("token revoked without being asked")
	}
	if code, out := e.portal(&ada, "POST", "/api/admin/users/"+cleo.user.ID+"/revoke-sessions", map[string]any{"tokens": true}); code != 200 || out["tokens"] != float64(1) {
		t.Fatalf("revoke sessions and tokens %d %v", code, out)
	}
	if code, _ := e.bearer(second, "GET", "/api/me", ""); code != 401 {
		t.Fatal("token survived revoke-sessions with tokens")
	}
}
