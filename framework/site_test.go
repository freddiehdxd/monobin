package framework

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedirect(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":       `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html": `{{ define "content" }}home{{ end }}`,
	})
	a.Redirect("/old", "/")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/old", nil))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
}

func TestCustom404(t *testing.T) {
	withPage := newAppFromFiles(t, false, map[string]string{
		"layout.html":       `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html": `{{ define "content" }}home{{ end }}`,
		"routes/404.html":   `{{ define "content" }}custom not found{{ end }}`,
	})
	if withPage.notFound == "" {
		t.Fatal("routes/404.html should be recorded as the custom 404 page, not a route")
	}
	if _, _, ok := withPage.match("/404"); ok {
		t.Error("/404 should not be a routable URL")
	}
	rec := httptest.NewRecorder()
	withPage.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "custom not found") {
		t.Errorf("expected custom 404 body, got %q", rec.Body.String())
	}

	// Without a 404.html, unmatched paths still get a plain 404.
	plain := newAppFromFiles(t, false, map[string]string{
		"layout.html":       `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html": `{{ define "content" }}home{{ end }}`,
	})
	rec2 := httptest.NewRecorder()
	plain.Handler().ServeHTTP(rec2, httptest.NewRequest("GET", "/nope", nil))
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec2.Code)
	}
}

func TestSitemap(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":             `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html":       `{{ define "content" }}h{{ end }}`,
		"routes/secret.html":      `{{ define "content" }}s{{ end }}`,
		"routes/blog/[slug].html": `{{ define "content" }}p{{ end }}`,
		"routes/x/[id].html":      `{{ define "content" }}x{{ end }}`, // no StaticPaths -> omitted
		"routes/bad/[id].html":    `{{ define "content" }}b{{ end }}`, // StaticPaths errors -> omitted
	})
	a.NoStatic("/secret")
	a.Meta("/", map[string]string{"changefreq": "weekly", "priority": "1.0"})
	a.StaticPaths("/blog/:slug", func() ([]map[string]string, error) {
		return []map[string]string{{"slug": "a"}}, nil
	})
	a.StaticPaths("/bad/:id", func() ([]map[string]string, error) {
		return nil, errors.New("boom") // must degrade, not sink the whole sitemap
	})

	sm, err := a.Sitemap("https://x.test")
	if err != nil {
		t.Fatal(err)
	}
	s := string(sm)
	for _, want := range []string{
		"https://x.test/", "https://x.test/blog/a",
		"<priority>1.0</priority>", "<changefreq>weekly</changefreq>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("sitemap missing %q:\n%s", want, s)
		}
	}
	for _, unwanted := range []string{"x.test/secret", "x.test/x/", "x.test/bad/"} {
		if strings.Contains(s, unwanted) {
			t.Errorf("sitemap should omit %q (NoStatic / no-StaticPaths / errored):\n%s", unwanted, s)
		}
	}
}

func TestBaseURLNormalizes(t *testing.T) {
	if got := (&App{SiteURL: "https://x.test/"}).baseURL(nil); got != "https://x.test" {
		t.Errorf("trailing slash not trimmed: %q", got)
	}
	if got := (&App{}).baseURL(nil); got != "https://example.com" {
		t.Errorf("build placeholder = %q, want https://example.com", got)
	}
}

func TestMetaInTemplate(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":       `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html": `{{ define "content" }}freq={{ .Meta.changefreq }}{{ end }}`,
	})
	a.Meta("/", map[string]string{"changefreq": "daily"})
	if out := renderPath(t, a, "/"); !strings.Contains(out, "freq=daily") {
		t.Errorf("Meta not exposed to template: %s", out)
	}
}
