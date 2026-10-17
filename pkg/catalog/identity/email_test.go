package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEmailProofRequiresMatchingFreshFactor(t *testing.T) {
	now := time.Now().UTC()
	for _, tt := range []struct {
		name, subject, org string
		verified, expiry   time.Time
		good               bool
	}{
		{"valid", "alice", "org", now, now.Add(time.Minute), true},
		{"user selection alone", "alice", "org", time.Time{}, now.Add(time.Minute), false},
		{"different user", "bob", "org", now, now.Add(time.Minute), false},
		{"different organization", "alice", "other", now, now.Add(time.Minute), false},
		{"old proof", "alice", "org", now.Add(-time.Hour), now.Add(time.Minute), false},
		{"future proof", "alice", "org", now.Add(time.Hour), now.Add(time.Minute), false},
		{"expired session", "alice", "org", now, now.Add(-time.Minute), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer private-token" {
					t.Error("missing credential")
				}
				if r.Method == "PATCH" {
					var in struct {
						Checks struct {
							Email struct {
								Code string `json:"code"`
							} `json:"otpEmail"`
						} `json:"checks"`
					}
					_ = json.NewDecoder(r.Body).Decode(&in)
					if in.Checks.Email.Code != "12345678" {
						t.Error("code not sent")
					}
					_, _ = w.Write([]byte(`{}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"session": map[string]any{"id": "sess", "expirationDate": tt.expiry, "factors": map[string]any{"user": map[string]string{"id": tt.subject, "organizationId": tt.org}, "otpEmail": map[string]any{"verifiedAt": tt.verified}}}})
			}))
			defer upstream.Close()
			p, err := NewEmailLogin(upstream.URL, "org", "private-token")
			if err != nil {
				t.Fatal(err)
			}
			err = p.Verify(context.Background(), "sess", "alice", "12345678", now.Add(-time.Minute))
			if (err == nil) != tt.good {
				t.Fatalf("Verify = %v; expected success %v", err, tt.good)
			}
		})
	}
}

func TestEmailLoginUserChecksAndDeliveryContract(t *testing.T) {
	active, verified, org, email := "USER_STATE_ACTIVE", true, "org", "alice@example.test"
	sent := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/users/alice":
			_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"userId": "alice", "state": active, "details": map[string]string{"resourceOwner": org}, "human": map[string]any{"email": map[string]any{"email": email, "isVerified": verified}}}})
		case "/v2/sessions":
			var in struct {
				Lifetime string `json:"lifetime"`
				Checks   struct {
					User struct {
						ID string `json:"userId"`
					} `json:"user"`
				} `json:"checks"`
				Challenges struct {
					Email struct {
						Send struct {
							URL string `json:"urlTemplate"`
						} `json:"sendCode"`
					} `json:"otpEmail"`
				} `json:"challenges"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
			}
			if in.Lifetime != "300s" || in.Checks.User.ID != "alice" || !strings.Contains(in.Challenges.Email.Send.URL, "{{.Code}}") {
				t.Errorf("invalid delivery request: %+v", in)
			}
			sent = true
			_, _ = w.Write([]byte(`{"sessionId":"sess","sessionToken":"must-not-be-exposed"}`))
		}
	}))
	defer upstream.Close()
	p, _ := NewEmailLogin(upstream.URL, "org", "private-token")
	if err := p.CheckUser(context.Background(), "alice", "ALICE@example.test"); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(){func() { active = "USER_STATE_INACTIVE" }, func() { active = "USER_STATE_ACTIVE"; verified = false }, func() { verified = true; org = "other" }, func() { org = "org"; email = "changed@example.test" }} {
		change()
		if err := p.CheckUser(context.Background(), "alice", "alice@example.test"); err == nil {
			t.Fatal("ineligible provider user accepted")
		}
	}
	id, err := p.SendLink(context.Background(), "alice", "https://portal.test/auth/email/verify?flow=abc&code={{.Code}}")
	if err != nil || id != "sess" || !sent {
		t.Fatalf("SendLink=%q %v", id, err)
	}
}

func TestEmailCredentialDoesNotFollowRedirectsOrLeakErrors(t *testing.T) {
	hit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer target.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/secret-code", http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	p, _ := NewEmailLogin(upstream.URL, "org", "private-token")
	err := p.CheckUser(context.Background(), "alice", "alice@example.test")
	if err == nil || hit || strings.Contains(err.Error(), "secret-code") || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("unsafe redirect/error: hit=%v err=%v", hit, err)
	}
	for _, issuer := range []string{"http://untrusted.example", "https://user:password@example.com", "https://example.com/?x=y"} {
		if _, err = NewEmailLogin(issuer, "org", "token"); err == nil {
			t.Errorf("accepted %s", issuer)
		}
	}
}
