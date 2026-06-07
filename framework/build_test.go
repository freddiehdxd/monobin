package framework

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildStatic exercises SSG end to end: static + dynamic expansion via
// StaticPaths, ErrNotFound paths skipped (not fatal), dynamic routes without
// StaticPaths skipped, and assets copied.
func TestBuildStatic(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":             `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html":       `{{ define "content" }}home{{ end }}`,
		"routes/blog/[slug].html": `{{ define "content" }}post {{ .Params.slug }}{{ end }}`,
		"routes/x/[id].html":      `{{ define "content" }}x{{ end }}`, // dynamic, NO StaticPaths -> skipped
		"assets/style.css":        `/* css */`,
		"assets/entry.js":         `// js`,
	})
	a.loaders["/blog/:slug"] = func(c *Ctx) (any, error) {
		if c.Params["slug"] == "missing" {
			return nil, ErrNotFound // must be skipped at build, not abort it
		}
		return nil, nil
	}
	a.staticPaths["/blog/:slug"] = func() ([]map[string]string, error) {
		return []map[string]string{{"slug": "a"}, {"slug": "missing"}, {"slug": "b"}}, nil
	}

	out := t.TempDir()
	if err := a.BuildStatic(out); err != nil {
		t.Fatalf("BuildStatic: %v", err)
	}

	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel)))
		return err == nil
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}

	if !exists("index.html") || !strings.Contains(read("index.html"), "home") {
		t.Error("missing or wrong dist/index.html")
	}
	if !strings.Contains(read("blog/a/index.html"), "post a") {
		t.Error("missing dist/blog/a/index.html")
	}
	if !strings.Contains(read("blog/b/index.html"), "post b") {
		t.Error("missing dist/blog/b/index.html")
	}
	if exists("blog/missing/index.html") {
		t.Error("ErrNotFound slug should be skipped, but dist/blog/missing/index.html exists")
	}
	if exists("x") {
		t.Error("dynamic route with no StaticPaths should produce no output")
	}
	if !exists("assets/style.css") || !exists("assets/entry.js") {
		t.Error("assets not copied into dist/assets")
	}
}

// TestValidateOutDir checks the destructive-target guard directly (no deletion).
func TestValidateOutDir(t *testing.T) {
	for _, bad := range []string{"", "   ", ".", "..", "/", string(filepath.Separator)} {
		if err := validateOutDir(bad); err == nil {
			t.Errorf("validateOutDir(%q) = nil, want refusal", bad)
		}
	}
	for _, ok := range []string{"dist", "build/site", filepath.Join(t.TempDir(), "out")} {
		if err := validateOutDir(ok); err != nil {
			t.Errorf("validateOutDir(%q) = %v, want nil", ok, err)
		}
	}
}
