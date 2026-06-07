package framework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

type renderData struct {
	Path   string
	Params map[string]string
	Data   any
}

// renderState is per-render scratch shared between the island and scripts
// template funcs. It lets {{ scripts }} (emitted last, in the layout) know
// whether any {{ island }} ran earlier on this page, so island-free pages can
// ship zero JavaScript.
type renderState struct {
	usedIsland bool
}

func (a *App) funcs(st *renderState) template.FuncMap {
	return template.FuncMap{
		// island emits an SSR placeholder; the client runtime hydrates it.
		//   {{ island "Counter" (dict "start" 5) }}
		"island": func(name string, props any) (template.HTML, error) {
			st.usedIsland = true
			b, err := json.Marshal(props)
			if err != nil {
				return "", err
			}
			return template.HTML(
				`<div data-island="` + template.HTMLEscapeString(name) +
					`" data-props="` + template.HTMLEscapeString(string(b)) + `"></div>`), nil
		},
		// scripts emits the client runtime tags. The layout calls it once, after
		// the content block, so usedIsland is accurate by the time it runs.
		"scripts": func() template.HTML {
			return a.islandScripts(st.usedIsland)
		},
		// styles links the prod stylesheet. In dev it's empty: the Vite entry
		// imports the CSS and injects it (with HMR), so there's no built file yet.
		"styles": func() template.HTML {
			return a.styleLinks()
		},
		"dict": func(kv ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(kv); i += 2 {
				if k, ok := kv[i].(string); ok {
					m[k] = kv[i+1]
				}
			}
			return m
		},
	}
}

// parse reads layout + route file directly (fs.ReadFile, not ParseFS) so that
// dynamic filenames like [slug].html aren't treated as glob patterns.
func (a *App) parse(tmplName string, st *renderState) (*template.Template, error) {
	// Prod: the embedded templates never change, so parse once and reuse a
	// prototype. Clone per request and rebind the FuncMap so the per-request
	// renderState stays isolated. Dev always re-reads from disk for live edits.
	if !a.Dev {
		if v, ok := a.tmplCache.Load(tmplName); ok {
			clone, err := v.(*template.Template).Clone()
			if err != nil {
				return nil, err
			}
			return clone.Funcs(a.funcs(st)), nil
		}
	}

	layout, err := fs.ReadFile(a.fsys, "layout.html")
	if err != nil {
		return nil, fmt.Errorf("monobin: reading app/layout.html: %w", err)
	}
	page, err := fs.ReadFile(a.fsys, tmplName)
	if err != nil {
		return nil, fmt.Errorf("monobin: reading template app/%s: %w", tmplName, err)
	}
	t := template.New("layout.html").Funcs(a.funcs(st))
	if _, err := t.Parse(string(layout)); err != nil {
		return nil, fmt.Errorf("monobin: parsing app/layout.html: %w — fix the template syntax", err)
	}
	if _, err := t.New("page").Parse(string(page)); err != nil {
		return nil, fmt.Errorf("monobin: parsing template app/%s: %w — fix the template syntax", tmplName, err)
	}
	if !a.Dev {
		// Cache an un-executed clone as the prototype (the returned t is about to
		// be executed by render, and Clone is illegal after Execute).
		if proto, cerr := t.Clone(); cerr == nil {
			a.tmplCache.Store(tmplName, proto)
		}
	}
	return t, nil
}

func (a *App) render(rt route, params map[string]string, r *http.Request) ([]byte, error) {
	st := &renderState{}
	t, err := a.parse(rt.tmplName, st)
	if err != nil {
		return nil, err
	}

	var data any
	if l, ok := a.loaders[rt.pattern]; ok {
		if data, err = l(&Ctx{Request: r, Params: params}); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, renderData{
		Path:   r.URL.Path,
		Params: params,
		Data:   data,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// islandScripts returns the client runtime tags for a page.
//   - dev: always wire the Vite client (HMR) + live-reload, even on island-free
//     pages, so editing any template/component refreshes the browser.
//   - prod: emit the island bundle ONLY if the page actually used an island.
//     Pages with no islands ship zero JavaScript.
func (a *App) islandScripts(usedIsland bool) template.HTML {
	if a.Dev {
		return template.HTML(
			`<script type="module" src="http://localhost:5173/@vite/client"></script>` +
				`<script type="module" src="http://localhost:5173/src/entry.js"></script>` +
				liveReloadScript)
	}
	if !usedIsland {
		return ""
	}
	return template.HTML(`<script type="module" src="/assets/entry.js"></script>`)
}

// styleLinks returns the stylesheet tag. In prod the compiled Tailwind sheet is
// linked on every page (CSS applies site-wide). In dev it's empty because the
// Vite entry imports style.css and injects it with hot reload.
func (a *App) styleLinks() template.HTML {
	if a.Dev {
		return ""
	}
	return template.HTML(`<link rel="stylesheet" href="/assets/style.css">`)
}
