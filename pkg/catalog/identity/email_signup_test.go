package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistrationRecoversOnlyItsReservedIdentityAfterConcurrentCreation(t *testing.T) {
	for _, mode := range []string{"create conflict", "search sees reserved", "search sees unrelated", "different organization", "changed email", "different subject", "inactive"} {
		t.Run(mode, func(t *testing.T) {
			gets, resends := 0, 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/users/reserved":
					gets++
					if gets == 1 {
						http.NotFound(w, r)
						return
					}
					subject, org, email, state := "reserved", "org", "new@example.test", "USER_STATE_ACTIVE"
					switch mode {
					case "different organization":
						org = "other"
					case "changed email":
						email = "changed@example.test"
					case "different subject":
						subject = "other"
					case "inactive":
						state = "USER_STATE_INACTIVE"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"userId": subject, "state": state, "details": map[string]string{"resourceOwner": org}, "human": map[string]any{"email": map[string]any{"email": email, "isVerified": false}}}})
				case "/v2/users":
					result := []map[string]string{}
					if mode == "search sees reserved" {
						result = append(result, map[string]string{"userId": "reserved"})
					}
					if mode == "search sees unrelated" {
						result = append(result, map[string]string{"userId": "unrelated"})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
				case "/v2/users/human":
					http.Error(w, "concurrent creation", http.StatusConflict)
				case "/v2/users/reserved/email/resend":
					var body struct {
						Send struct {
							URL string `json:"urlTemplate"`
						} `json:"sendCode"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body.Send.URL != "https://portal.test/auth/email/verify?flow=current&code={{.Code}}" {
						t.Error("did not resend the current flow")
					}
					resends++
					_, _ = w.Write([]byte(`{}`))
				default:
					t.Errorf("unexpected provider mutation: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			client, err := NewEmailLogin(upstream.URL, "org", "test-token")
			if err != nil {
				t.Fatal(err)
			}
			session, err := client.SendRegistration(context.Background(), "reserved", "new@example.test", "https://portal.test/auth/email/verify?flow=current&code={{.Code}}")
			good := mode == "create conflict" || mode == "search sees reserved"
			if (err == nil) != good || session != "" {
				t.Fatalf("recovery success %v: %v", good, err)
			}
			if good && (gets != 2 || resends != 1) {
				t.Fatalf("recovery calls: reads %d resends %d", gets, resends)
			}
			if !good && resends != 0 {
				t.Fatal("sent a link to an ineligible identity")
			}
		})
	}
}
