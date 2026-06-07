package framework

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
)

// ErrNotFound lets a loader signal a 404 (e.g. unknown :slug).
var ErrNotFound = errors.New("not found")

// Ctx is passed to every loader. Params holds dynamic route segments,
// e.g. for /blog/:slug, Params["slug"] is the matched value.
type Ctx struct {
	Request *http.Request
	Params  map[string]string
}

// Loader runs server-side before render; its return value is the template's .Data.
type Loader func(c *Ctx) (any, error)

// StaticPaths enumerates the concrete param sets to pre-render for a dynamic
// route at build time (like Next's getStaticPaths / Astro's getStaticPaths).
type StaticPaths func() ([]map[string]string, error)

type segment struct {
	value string // literal text, or param name when param==true
	param bool
}

type route struct {
	pattern  string // "/", "/about", "/blog/:slug" — also the loader key
	segs     []segment
	dynamic  bool
	tmplName string // literal path under app/, e.g. "routes/blog/[slug].html"
}

type App struct {
	Dev         bool
	fsys        fs.FS // app/ — disk in dev (live edits), embed in prod
	routes      []route
	loaders     map[string]Loader
	staticPaths map[string]StaticPaths
}

// New builds an App. Dev reads app/ from disk (live reload, no recompile);
// prod uses the embedded copy baked into the binary.
func New(embedded embed.FS, dev bool) (*App, error) {
	var fsys fs.FS
	if dev {
		fsys = os.DirFS("app")
	} else {
		sub, err := fs.Sub(embedded, "app")
		if err != nil {
			return nil, err
		}
		fsys = sub
	}

	a := &App{
		Dev:         dev,
		fsys:        fsys,
		loaders:     map[string]Loader{},
		staticPaths: map[string]StaticPaths{},
	}
	if err := a.scanRoutes(); err != nil {
		return nil, err
	}
	a.registerLoaders()
	return a, nil
}

// scanRoutes walks routes/**/*.html and maps files to URL patterns.
//
//	routes/index.html        -> /
//	routes/about.html        -> /about
//	routes/blog/index.html   -> /blog
//	routes/blog/[slug].html  -> /blog/:slug   (dynamic)
func (a *App) scanRoutes() error {
	a.routes = nil
	err := fs.WalkDir(a.fsys, "routes", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".html") {
			return nil
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(p, "routes"), ".html")
		rel = strings.TrimSuffix(rel, "/index")
		if rel == "" {
			rel = "/"
		}
		a.routes = append(a.routes, makeRoute(rel, p))
		return nil
	})
	if err != nil {
		return err
	}
	// Static routes are matched before dynamic ones so /blog/featured beats
	// /blog/:slug when both exist.
	sort.SliceStable(a.routes, func(i, j int) bool {
		return !a.routes[i].dynamic && a.routes[j].dynamic
	})
	return nil
}

func makeRoute(rel, tmpl string) route {
	if rel == "/" {
		return route{pattern: "/", tmplName: tmpl}
	}
	raw := strings.Split(strings.Trim(rel, "/"), "/")
	segs := make([]segment, 0, len(raw))
	disp := make([]string, 0, len(raw))
	dyn := false
	for _, r := range raw {
		if strings.HasPrefix(r, "[") && strings.HasSuffix(r, "]") {
			name := r[1 : len(r)-1]
			segs = append(segs, segment{value: name, param: true})
			disp = append(disp, ":"+name)
			dyn = true
		} else {
			segs = append(segs, segment{value: r})
			disp = append(disp, r)
		}
	}
	return route{pattern: "/" + strings.Join(disp, "/"), segs: segs, dynamic: dyn, tmplName: tmpl}
}

// match resolves a URL path to a route and extracts any params.
func (a *App) match(urlPath string) (route, map[string]string, bool) {
	var parts []string
	if urlPath != "/" {
		parts = strings.Split(strings.Trim(urlPath, "/"), "/")
	}
	for _, rt := range a.routes {
		if len(rt.segs) != len(parts) {
			continue
		}
		params := map[string]string{}
		ok := true
		for i, s := range rt.segs {
			if s.param {
				params[s.value] = parts[i]
			} else if s.value != parts[i] {
				ok = false
				break
			}
		}
		if ok {
			return rt, params, true
		}
	}
	return route{}, nil, false
}

// fillPattern turns a route + params into a concrete URL (for SSG output).
func fillPattern(rt route, params map[string]string) string {
	if len(rt.segs) == 0 {
		return "/"
	}
	out := make([]string, 0, len(rt.segs))
	for _, s := range rt.segs {
		if s.param {
			out = append(out, params[s.value])
		} else {
			out = append(out, s.value)
		}
	}
	return "/" + strings.Join(out, "/")
}
