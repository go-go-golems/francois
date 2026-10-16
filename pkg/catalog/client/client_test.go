package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRetriesIdempotentRequestsOnly(t *testing.T) {
	var gets, posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer frt_test" {
			t.Errorf("authorization header %q", r.Header.Get("Authorization"))
		}
		n := &gets
		if r.Method == http.MethodPost {
			n = &posts
		}
		if n.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"user":{"id":"usr_1","name":"A","email":"a@x","role":"admin","disabled":false},"token":null,"creator":null}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "frt_test")
	me, err := c.Me(context.Background())
	if err != nil || me.User.ID != "usr_1" || gets.Load() != 3 {
		t.Fatalf("GET after two 503s: %+v %v (attempts %d)", me, err, gets.Load())
	}
	err = c.Studio.PublishDraft(context.Background(), "x")
	if !IsCode(err, "http_503") || posts.Load() != 1 {
		t.Fatalf("POST must not be retried: %v (attempts %d)", err, posts.Load())
	}
}

func TestDecodesErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_release","message":"bad html"},"checks":[{"code":"no_html","level":"error","message":"not a document"}]}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL, "frt_test").Studio.CreateRelease(context.Background(), ReleaseUpload{ArtifactID: "a", Version: "1", HTML: []byte("x")})
	e, ok := err.(*Error)
	if !ok || e.Status != 422 || e.Code != "invalid_release" || len(e.Checks) != 1 || e.Error() != "invalid_release: bad html" {
		t.Fatalf("error %#v", err)
	}
}
