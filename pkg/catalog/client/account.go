package client

import (
	"context"
	"encoding/json"
	"net/http"
)

// Account is the caller's own account (scope account): account details,
// Stripe Checkout and portal links, and instances. Launching an instance is
// browser-only.
type Account struct{ c *Client }

// Get returns the account: entitlement, subscriptions, grants, allowance.
func (a Account) Get(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, "/api/account", nil, &out)
}

// Checkout returns a Stripe Checkout URL for a plan and interval (month, year).
func (a Account) Checkout(ctx context.Context, plan, interval string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	return out.URL, a.c.send2(ctx, http.MethodPost, "/api/billing/checkout", map[string]string{"plan": plan, "interval": interval}, &out)
}

// Portal returns a Stripe customer portal URL. The URL grants access to the
// owner's billing; treat it like a password.
func (a Account) Portal(ctx context.Context) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	return out.URL, a.c.send2(ctx, http.MethodPost, "/api/billing/portal", nil, &out)
}

func (a Account) Instances(ctx context.Context) ([]Instance, error) {
	var out []Instance
	return out, a.c.get(ctx, "/api/instances", nil, &out)
}

func (a Account) Instance(ctx context.Context, id string) (Instance, error) {
	var out Instance
	return out, a.c.get(ctx, "/api/instances/"+esc(id), nil, &out)
}

func (a Account) CreateInstance(ctx context.Context, releaseID, title string) (Instance, error) {
	var out Instance
	return out, a.c.send2(ctx, http.MethodPost, "/api/instances", map[string]string{"releaseId": releaseID, "title": title}, &out)
}

func (a Account) RenameInstance(ctx context.Context, id, title string) error {
	return a.c.send2(ctx, http.MethodPatch, "/api/instances/"+esc(id), map[string]string{"title": title}, nil)
}

func (a Account) DeleteInstance(ctx context.Context, id string) error {
	return a.c.send2(ctx, http.MethodDelete, "/api/instances/"+esc(id), nil, nil)
}

// AddMember shares an instance with another subscriber (role editor or viewer).
func (a Account) AddMember(ctx context.Context, id, email, role string) (Member, error) {
	var out Member
	return out, a.c.send2(ctx, http.MethodPost, "/api/instances/"+esc(id)+"/members", map[string]string{"email": email, "role": role}, &out)
}

func (a Account) RemoveMember(ctx context.Context, id, userID string) error {
	return a.c.send2(ctx, http.MethodDelete, "/api/instances/"+esc(id)+"/members/"+esc(userID), nil, nil)
}

// Upgrade moves an instance to a newer release of its artifact.
func (a Account) Upgrade(ctx context.Context, id, releaseID string) error {
	return a.c.send2(ctx, http.MethodPost, "/api/instances/"+esc(id)+"/upgrade", map[string]string{"releaseId": releaseID}, nil)
}
