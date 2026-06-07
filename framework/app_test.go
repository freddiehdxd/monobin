package framework

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// newTestApp builds an App backed by an in-memory routes tree and runs the
// REAL scan+sort pipeline, so match() is exercised exactly as in production
// (including static-before-dynamic ordering). The [slug] filenames double as
// a guard that fs.WalkDir treats brackets literally, not as glob patterns.
func newTestApp(t *testing.T, routePaths ...string) *App {
	t.Helper()
	files := fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte("layout")},
	}
	for _, p := range routePaths {
		files[p] = &fstest.MapFile{Data: []byte("page")}
	}
	a := &App{
		fsys:        files,
		loaders:     map[string]Loader{},
		staticPaths: map[string]StaticPaths{},
		staticSkip:  map[string]bool{},
		redirects:   map[string]string{},
		meta:        map[string]map[string]string{},
	}
	if err := a.scanRoutes(); err != nil {
		t.Fatalf("scanRoutes: %v", err)
	}
	return a
}

func TestMakeRoute(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		wantPat  string
		wantDyn  bool
		wantSegs int
	}{
		{"root", "/", "/", false, 0},
		{"static one segment", "/about", "/about", false, 1},
		{"static nested index", "/blog", "/blog", false, 1},
		{"dynamic single param", "/blog/[slug]", "/blog/:slug", true, 2},
		{"dynamic multi param", "/docs/[section]/[page]", "/docs/:section/:page", true, 3},
		{"mixed static + param", "/users/[id]/profile", "/users/:id/profile", true, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := makeRoute(tt.rel, "routes"+tt.rel+".html")
			if rt.pattern != tt.wantPat {
				t.Errorf("pattern = %q, want %q", rt.pattern, tt.wantPat)
			}
			if rt.dynamic != tt.wantDyn {
				t.Errorf("dynamic = %v, want %v", rt.dynamic, tt.wantDyn)
			}
			if len(rt.segs) != tt.wantSegs {
				t.Errorf("len(segs) = %d, want %d", len(rt.segs), tt.wantSegs)
			}
		})
	}
}

func TestMatch(t *testing.T) {
	a := newTestApp(t,
		"routes/index.html",         // /
		"routes/about.html",         // /about
		"routes/blog/index.html",    // /blog
		"routes/blog/featured.html", // /blog/featured  (static)
		"routes/blog/[slug].html",   // /blog/:slug     (dynamic)
	)

	tests := []struct {
		name       string
		path       string
		wantOK     bool
		wantPat    string
		wantParams map[string]string
	}{
		{"root path", "/", true, "/", map[string]string{}},
		{"static one segment", "/about", true, "/about", map[string]string{}},
		{"static nested index", "/blog", true, "/blog", map[string]string{}},
		{"dynamic match + param capture", "/blog/hello-world", true, "/blog/:slug", map[string]string{"slug": "hello-world"}},
		{"static beats dynamic", "/blog/featured", true, "/blog/featured", map[string]string{}},
		{"no match: unknown path", "/nope", false, "", nil},
		{"no match: too deep", "/blog/a/b", false, "", nil},
		{"no match: empty segment depth", "/about/extra", false, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, params, ok := a.match(tt.path)
			if ok != tt.wantOK {
				t.Fatalf("match(%q) ok = %v, want %v", tt.path, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if rt.pattern != tt.wantPat {
				t.Errorf("match(%q) pattern = %q, want %q", tt.path, rt.pattern, tt.wantPat)
			}
			if !reflect.DeepEqual(params, tt.wantParams) {
				t.Errorf("match(%q) params = %#v, want %#v", tt.path, params, tt.wantParams)
			}
		})
	}
}

// TestStaticBeatsDynamicOrdering pins the invariant that scanRoutes sorts all
// static routes ahead of dynamic ones regardless of filesystem walk order.
func TestStaticBeatsDynamicOrdering(t *testing.T) {
	a := newTestApp(t,
		"routes/blog/[slug].html",   // dynamic first on disk...
		"routes/blog/featured.html", // ...static second
	)
	rt, _, ok := a.match("/blog/featured")
	if !ok {
		t.Fatal(`match("/blog/featured") = no match, want the static route`)
	}
	if rt.dynamic || rt.pattern != "/blog/featured" {
		t.Errorf("matched %q (dynamic=%v), want static /blog/featured", rt.pattern, rt.dynamic)
	}
}

// TestRouteCollision pins the fail-fast behavior: two files normalizing to the
// same URL pattern must error during scan, not silently shadow each other.
func TestRouteCollision(t *testing.T) {
	mfs := fstest.MapFS{
		"layout.html":            &fstest.MapFile{Data: []byte("layout")},
		"routes/blog.html":       &fstest.MapFile{Data: []byte("a")},
		"routes/blog/index.html": &fstest.MapFile{Data: []byte("b")},
	}
	a := &App{fsys: mfs, loaders: map[string]Loader{}, staticPaths: map[string]StaticPaths{}}
	err := a.scanRoutes()
	if err == nil {
		t.Fatal("expected a route conflict error for blog.html vs blog/index.html, got nil")
	}
	if !strings.Contains(err.Error(), "/blog") {
		t.Errorf("conflict error should name the pattern /blog, got: %v", err)
	}
}

// TestMatchTrailingSlash characterizes current (lenient) trailing-slash matching
// so a future refactor of match() can't change it silently.
func TestMatchTrailingSlash(t *testing.T) {
	a := newTestApp(t,
		"routes/index.html",
		"routes/about.html",
		"routes/blog/[slug].html",
	)
	cases := []struct{ path, pattern string }{
		{"/about/", "/about"},
		{"/blog/hello/", "/blog/:slug"},
	}
	for _, c := range cases {
		rt, _, ok := a.match(c.path)
		if !ok || rt.pattern != c.pattern {
			t.Errorf("match(%q) = (%q, ok=%v), want (%q, true)", c.path, rt.pattern, ok, c.pattern)
		}
	}
}

func TestFillPattern(t *testing.T) {
	tests := []struct {
		name   string
		rel    string
		params map[string]string
		want   string
	}{
		{"root", "/", nil, "/"},
		{"static", "/about", nil, "/about"},
		{"single param", "/blog/[slug]", map[string]string{"slug": "hello-world"}, "/blog/hello-world"},
		{"multi param", "/docs/[section]/[page]", map[string]string{"section": "guide", "page": "intro"}, "/docs/guide/intro"},
		{"mixed static + param", "/users/[id]/profile", map[string]string{"id": "42"}, "/users/42/profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := makeRoute(tt.rel, "routes"+tt.rel+".html")
			got := fillPattern(rt, tt.params)
			if got != tt.want {
				t.Errorf("fillPattern(%q) = %q, want %q", tt.rel, got, tt.want)
			}
		})
	}
}

// TestFillPatternRoundTrip proves fillPattern and match are inverses: building
// a concrete URL from params and matching it back recovers the same route and
// params. This is the SSG <-> runtime contract.
func TestFillPatternRoundTrip(t *testing.T) {
	a := newTestApp(t,
		"routes/index.html",
		"routes/blog/[slug].html",
		"routes/docs/[section]/[page].html",
		"routes/users/[id]/profile.html",
	)
	cases := []struct {
		pattern string
		params  map[string]string
	}{
		{"/", map[string]string{}},
		{"/blog/:slug", map[string]string{"slug": "hello-world"}},
		{"/docs/:section/:page", map[string]string{"section": "guide", "page": "intro"}},
		{"/users/:id/profile", map[string]string{"id": "42"}},
	}
	for _, c := range cases {
		t.Run(c.pattern, func(t *testing.T) {
			var rt route
			found := false
			for _, r := range a.routes {
				if r.pattern == c.pattern {
					rt, found = r, true
					break
				}
			}
			if !found {
				t.Fatalf("no route registered for pattern %q", c.pattern)
			}

			url := fillPattern(rt, c.params)
			gotRt, gotParams, ok := a.match(url)
			if !ok {
				t.Fatalf("match(%q) failed to round-trip", url)
			}
			if gotRt.pattern != c.pattern {
				t.Errorf("round-trip pattern = %q, want %q", gotRt.pattern, c.pattern)
			}
			if !reflect.DeepEqual(gotParams, c.params) {
				t.Errorf("round-trip params = %#v, want %#v", gotParams, c.params)
			}
		})
	}
}
