package hosted

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/catalog/web"
)

func TestBootstrapCachePolicy(t *testing.T) {
	rt := &Runtime{}
	h := rt.Handler(store.Instance{Title: "Example"}, "https://portal.test", "https://instance.test")
	for _, name := range []string{"launch.js", "launch.css", "runtime.js"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", web.AppURL(name), nil))
		if w.Code != 200 || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") || w.Header().Get("ETag") == "" {
			t.Fatal(name, w.Code, w.Header())
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/launch", nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), web.AppURL("launch.js")) {
		t.Fatal("launch document must remain uncached and reference versioned assets", w.Code, w.Header())
	}
}
