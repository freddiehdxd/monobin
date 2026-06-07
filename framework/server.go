package framework

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

// Serve starts the HTTP server: matched routes -> SSR, /assets/ -> embedded build.
func (a *App) Serve(addr string) error {
	return http.ListenAndServe(addr, a.Handler())
}

// Handler builds the full HTTP handler (assets, dev live-reload, and the
// middleware-wrapped route renderer). Exposed so tests and embedders can drive
// the app without binding a port.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	assetFS, _ := fs.Sub(a.fsys, "assets")
	files := http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS)))
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r) // no directory listings of embedded assets
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		files.ServeHTTP(w, r)
	})

	if a.Dev {
		mux.HandleFunc("/__live", a.liveReload)
	}

	// Match first so the route pattern is in context before middleware runs,
	// then hand off to the middleware-wrapped renderer.
	render := a.chain(http.HandlerFunc(a.handleRoute))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if rt, params, ok := a.match(r.URL.Path); ok {
			r = withMatch(r, matched{rt, params})
		}
		render.ServeHTTP(w, r)
	})

	return mux
}

// handleRoute renders the route resolved by Handler (pulled from context). It is
// the innermost handler in the middleware chain; a middleware that writes a
// response and does not call next short-circuits before this runs.
func (a *App) handleRoute(w http.ResponseWriter, r *http.Request) {
	m, ok := matchFrom(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	html, err := a.render(m.rt, m.params, r)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		// Log the detail for the operator/agent; don't leak internals to the client.
		log.Printf("monobin: rendering %s (%s): %v", r.URL.Path, m.rt.tmplName, err)
		msg := "internal server error"
		if a.Dev {
			msg = err.Error()
		}
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(html)
}

// --- dev live reload (zero-dependency: SSE + mtime poll) ---

const liveReloadScript = `<script>
new EventSource('/__live').onmessage = function(e){ if(e.data==='reload') location.reload(); };
</script>`

func (a *App) liveReload(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	fl.Flush() // open the stream on connect so EventSource is ready before the first change

	last := a.latestMod()
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if m := a.latestMod(); m.After(last) {
				last = m
				w.Write([]byte("data: reload\n\n"))
				fl.Flush()
			}
		}
	}
}

func (a *App) latestMod() time.Time {
	var newest time.Time
	fs.WalkDir(a.fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p == "assets" {
				return fs.SkipDir // dev reload watches templates, not built assets
			}
			return nil
		}
		if info, e := d.Info(); e == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}
