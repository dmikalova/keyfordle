package site_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/dmikalova/keyfordle/internal/site"
)

func built() fstest.MapFS {
	return fstest.MapFS{
		"assets/app/index.html":         {Data: []byte("<!doctype html><title>KeyFordle</title>")},
		"assets/app/static/app-1a2b.js": {Data: []byte("console.log(1)")},
		"assets/app/assets/Aember.png":  {Data: []byte("png")},
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

func TestCacheHeaders(t *testing.T) {
	h := site.New(built())

	tests := []struct {
		path, cache, body string
	}{
		{"/", "no-cache", "<!doctype html><title>KeyFordle</title>"},
		{"/index.html", "no-cache", "<!doctype html><title>KeyFordle</title>"},
		{"/keyfordle", "no-cache", "<!doctype html><title>KeyFordle</title>"},
		{"/static", "no-cache", "<!doctype html><title>KeyFordle</title>"},
		{"/static/app-1a2b.js", "public, max-age=31536000, immutable", "console.log(1)"},
		{"/assets/Aember.png", "public, max-age=86400", "png"},
	}

	for _, tt := range tests {
		rec := get(t, h, tt.path)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", tt.path, rec.Code)
		}

		if got := rec.Header().Get("Cache-Control"); got != tt.cache {
			t.Errorf("GET %s Cache-Control = %q, want %q", tt.path, got, tt.cache)
		}

		if rec.Body.String() != tt.body {
			t.Errorf("GET %s body = %q", tt.path, rec.Body.String())
		}
	}
}

func TestUnbuilt(t *testing.T) {
	rec := get(t, site.New(fstest.MapFS{}), "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET / without a build = %d, want 503", rec.Code)
	}
}

func TestEmbeddedHandlerServes(t *testing.T) {
	// The embedded build may or may not exist, so either answer is valid; the
	// test only checks that the handler does not panic and answers.
	rec := get(t, site.Handler(), "/")
	if rec.Code != http.StatusOK && rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET / = %d", rec.Code)
	}
}
