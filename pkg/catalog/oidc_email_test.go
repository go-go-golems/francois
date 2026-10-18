package catalog

import (
	"context"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"testing"
)

func TestOIDCRefreshCannotTakeAnotherAccountsEmail(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	owner, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: "issuer", Subject: "owner", Email: "owned@example.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	other, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: "issuer", Subject: "other", Email: "other@example.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.store.UpsertIdentity(ctx, store.Identity{Issuer: other.Issuer, Subject: other.Subject, Email: "OWNED@example.test", EmailVerified: true})
	if err != store.ErrNotFound {
		t.Fatalf("colliding refresh %v", err)
	}
	unchanged, err := e.store.User(ctx, other.ID)
	if err != nil || unchanged.Email != other.Email {
		t.Fatalf("profile changed despite collision: %+v %v", unchanged, err)
	}
	users, err := e.store.UsersByEmail(ctx, owner.Email)
	if err != nil || len(users) != 1 {
		t.Fatalf("email owners %d %v", len(users), err)
	}
}

func TestOIDCWithoutEmailDoesNotCollideWithGuests(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.store.CreateGuest(ctx); err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"no-email-one", "no-email-two"} {
		id := store.Identity{Issuer: "issuer", Subject: subject, Name: "Initial"}
		u, err := e.store.UpsertIdentity(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		id.Name = "Refreshed"
		updated, err := e.store.UpsertIdentity(ctx, id)
		if err != nil || updated.ID != u.ID || updated.DisplayName != "Refreshed" || updated.Email != "" {
			t.Fatalf("missing-email refresh %+v %v", updated, err)
		}
	}
}
