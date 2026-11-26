// Package site serves the built React page.
//
// `vp run build` writes the Vite build to internal/site/assets/app, which the
// binary embeds. The directory is not committed. Without it the binary still
// builds, and every page request gets a 503 that says to build the frontend.
package site

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:assets
var assets embed.FS

// Cache lifetimes. Vite puts content-hashed files under static/, so a new build
// never reuses a name and they can be cached for a year. Files copied from
// web/public keep their names, so they get a day. The page itself is always
// revalidated, so a deploy reaches players on their next load.
const (
	cacheHashed   = "public, max-age=31536000, immutable"
	cachePublic   = "public, max-age=86400"
	cachePage     = "no-cache"
	hashedDir     = "static/"
	indexFileName = "index.html"
)

// Handler serves the embedded page. A path that names no file gets index.html,
// so the page can own its own routes.
func Handler() http.Handler { return New(assets) }

// New serves the page built into fsys under assets/app.
func New(fsys fs.FS) http.Handler {
	app, err := fs.Sub(fsys, "assets/app")
	if err != nil {
		return unbuilt()
	}

	index, err := fs.ReadFile(app, indexFileName)
	if err != nil {
		return unbuilt()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

		if name != "" && name != indexFileName {
			if serveFile(w, r, app, name) {
				return
			}
		}

		w.Header().Set("Cache-Control", cachePage)
		http.ServeContent(w, r, indexFileName, time.Time{}, bytes.NewReader(index))
	})
}

// serveFile serves name from app and reports whether it existed.
func serveFile(w http.ResponseWriter, r *http.Request, app fs.FS, name string) bool {
	info, err := fs.Stat(app, name)
	if err != nil || info.IsDir() {
		return false
	}

	if strings.HasPrefix(name, hashedDir) {
		w.Header().Set("Cache-Control", cacheHashed)
	} else {
		w.Header().Set("Cache-Control", cachePublic)
	}

	http.ServeFileFS(w, r, app, name)

	return true
}

// errUnbuilt is the body of every page request when the frontend is missing.
var errUnbuilt = errors.New(
	"the frontend is not built: run `vp run build`, then rebuild the binary",
)

func unbuilt() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, errUnbuilt.Error(), http.StatusServiceUnavailable)
	})
}
