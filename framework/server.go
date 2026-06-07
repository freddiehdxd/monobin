package framework

import (
	"errors"
	"io/fs"
	"net/http"
	"time"
)

// Serve starts the HTTP server: matched routes -> SSR, /assets/ -> embedded build.
func (a *App) Serve(addr string) error {
	mux := http.NewServeMux()

	assetFS, _ := fs.Sub(a.fsys, "assets")
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS))))

	if a.Dev {
		mux.HandleFunc("/__live", a.liveReload)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rt, params, ok := a.match(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		html, err := a.render(rt, params, r)
		if errors.Is(err, ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
	})

	return http.ListenAndServe(addr, mux)
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
		if err != nil || d.IsDir() {
			return nil
		}
		if info, e := d.Info(); e == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}
