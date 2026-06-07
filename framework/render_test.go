package framework

import (
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

// newAppFromFiles builds an App over an in-memory file set and runs the real
// scan pipeline. Caller sets Dev and registers loaders/staticPaths as needed.
func newAppFromFiles(t *testing.T, dev bool, files map[string]string) *App {
	t.Helper()
	mfs := fstest.MapFS{}
	for name, body := range files {
		mfs[name] = &fstest.MapFile{Data: []byte(body)}
	}
	a := &App{
		Dev:         dev,
		fsys:        mfs,
		loaders:     map[string]Loader{},
		staticPaths: map[string]StaticPaths{},
	}
	if err := a.scanRoutes(); err != nil {
		t.Fatalf("scanRoutes: %v", err)
	}
	return a
}

func renderPath(t *testing.T, a *App, path string) string {
	t.Helper()
	rt, params, ok := a.match(path)
	if !ok {
		t.Fatalf("match(%q) = no route", path)
	}
	out, err := a.render(rt, params, httptest.NewRequest("GET", path, nil))
	if err != nil {
		t.Fatalf("render(%q): %v", path, err)
	}
	return string(out)
}

const testLayout = `<!doctype html><html><head>{{ styles }}</head><body>{{ block "content" . }}{{ end }}{{ scripts }}</body></html>`

// TestRenderConditionalScriptsProd is the framework's headline invariant:
// island pages ship the bundle (+ an HTML-escaped placeholder), island-free
// pages ship zero JS, and both get the stylesheet.
func TestRenderConditionalScriptsProd(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":       testLayout,
		"routes/index.html": `{{ define "content" }}<h1>{{ .Data.Title }}</h1>{{ island "Counter" (dict "start" 3) }}{{ end }}`,
		"routes/about.html": `{{ define "content" }}<h1>about</h1>{{ end }}`,
	})
	a.loaders["/"] = func(c *Ctx) (any, error) { return map[string]any{"Title": "Home"}, nil }

	island := renderPath(t, a, "/")
	plain := renderPath(t, a, "/about")

	if !strings.Contains(island, `data-island="Counter"`) {
		t.Errorf("island page missing placeholder:\n%s", island)
	}
	// JSON quotes must be HTML-escaped inside the attribute (XSS / attribute breakout guard).
	if !strings.Contains(island, `data-props="{&#34;start&#34;:3}"`) {
		t.Errorf("island props not HTML-escaped:\n%s", island)
	}
	if !strings.Contains(island, `src="/assets/entry.js"`) {
		t.Errorf("island page should ship the bundle:\n%s", island)
	}
	if strings.Contains(plain, "entry.js") {
		t.Errorf("island-free page must ship ZERO JS:\n%s", plain)
	}
	if !strings.Contains(island, "/assets/style.css") || !strings.Contains(plain, "/assets/style.css") {
		t.Errorf("both pages should link the prod stylesheet")
	}
}

// TestRenderDevInjects verifies dev wires Vite HMR + live-reload on every page
// (even island-free) and does NOT link the prod stylesheet.
func TestRenderDevInjects(t *testing.T) {
	a := newAppFromFiles(t, true, map[string]string{
		"layout.html":       testLayout,
		"routes/about.html": `{{ define "content" }}<h1>about</h1>{{ end }}`,
	})
	out := renderPath(t, a, "/about")
	for _, want := range []string{
		"http://localhost:5173/@vite/client",
		"http://localhost:5173/src/entry.js",
		"EventSource('/__live')",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dev page missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/assets/style.css") {
		t.Errorf("dev should not link the prod stylesheet (Vite injects CSS):\n%s", out)
	}
}

// TestRenderLoaderWiring confirms the loader (keyed by route pattern) reaches the
// template as .Data, alongside .Params and .Path.
func TestRenderLoaderWiring(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":             testLayout,
		"routes/post/[slug].html": `{{ define "content" }}data={{ .Data.Msg }} slug={{ .Params.slug }} path={{ .Path }}{{ end }}`,
	})
	a.loaders["/post/:slug"] = func(c *Ctx) (any, error) {
		return map[string]any{"Msg": "hi"}, nil
	}
	out := renderPath(t, a, "/post/world")
	if !strings.Contains(out, "data=hi slug=world path=/post/world") {
		t.Errorf("loader Data / Params / Path not wired through:\n%s", out)
	}
}

// TestRenderConcurrent hammers the prod template cache from many goroutines
// (run under -race) and checks each render still produces the right output —
// island page ships the bundle, island-free page ships zero JS.
func TestRenderConcurrent(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":       testLayout,
		"routes/index.html": `{{ define "content" }}<h1>{{ .Data.Title }}</h1>{{ island "Counter" (dict "start" 3) }}{{ end }}`,
		"routes/about.html": `{{ define "content" }}<h1>about</h1>{{ end }}`,
	})
	a.loaders["/"] = func(c *Ctx) (any, error) { return map[string]any{"Title": "Home"}, nil }

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path, wantJS := "/about", false
			if i%2 == 0 {
				path, wantJS = "/", true
			}
			rt, params, ok := a.match(path)
			if !ok {
				t.Errorf("no match for %s", path)
				return
			}
			b, err := a.render(rt, params, httptest.NewRequest("GET", path, nil))
			if err != nil {
				t.Errorf("render %s: %v", path, err)
				return
			}
			if has := strings.Contains(string(b), "entry.js"); has != wantJS {
				t.Errorf("%s entry.js=%v, want %v", path, has, wantJS)
			}
		}(i)
	}
	wg.Wait()
}

// TestRenderErrNotFoundPropagates locks the contract server.go relies on for 404s:
// render must return ErrNotFound unwrapped.
func TestRenderErrNotFoundPropagates(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":             testLayout,
		"routes/post/[slug].html": `{{ define "content" }}x{{ end }}`,
	})
	a.loaders["/post/:slug"] = func(c *Ctx) (any, error) { return nil, ErrNotFound }
	rt, params, _ := a.match("/post/missing")
	_, err := a.render(rt, params, httptest.NewRequest("GET", "/post/missing", nil))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("render should surface ErrNotFound unwrapped, got %v", err)
	}
}
