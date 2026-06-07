package framework

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
)

// BuildStatic renders every route to static HTML under outDir. Dynamic routes
// are expanded via their registered StaticPaths. Output is a plain folder you
// can serve from anywhere (Caddy file_server, R2, a CDN).
func (a *App) BuildStatic(outDir string) error {
	// BuildStatic wipes outDir first; guard against a typo nuking the project
	// (e.g. `monobin build .` / an existing dir / `/`), since users run the
	// single binary from their own shell with an arbitrary arg.
	if err := validateOutDir(outDir); err != nil {
		return err
	}
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	for _, rt := range a.routes {
		if a.staticSkip[rt.pattern] {
			fmt.Printf("  skip %s (SSR-only: marked NoStatic, e.g. an auth-gated page)\n", rt.pattern)
			continue
		}
		if rt.dynamic {
			sp, ok := a.staticPaths[rt.pattern]
			if !ok {
				fmt.Printf("monobin: route %s is dynamic but has no StaticPaths registered — add a.staticPaths[%q] or it is skipped by 'monobin build'\n", rt.pattern, rt.pattern)
				continue
			}
			sets, err := sp()
			if err != nil {
				return err
			}
			for _, params := range sets {
				if err := a.writeRoute(outDir, rt, params); err != nil {
					return err
				}
			}
		} else if err := a.writeRoute(outDir, rt, nil); err != nil {
			return err
		}
	}
	return a.copyAssets(filepath.Join(outDir, "assets"))
}

// validateOutDir refuses obviously destructive build targets: empty, ".", "..",
// the filesystem root, or the current working directory / any ancestor of it
// (deleting which would take the source tree with it).
func validateOutDir(outDir string) error {
	if strings.TrimSpace(outDir) == "" {
		return errors.New("build: output directory is empty")
	}
	clean := filepath.Clean(outDir)
	switch clean {
	case ".", "..", string(filepath.Separator):
		return fmt.Errorf("build: refusing to delete %q", outDir)
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return err
	}
	if abs == filepath.Dir(abs) { // filesystem root (e.g. "/" or "C:\")
		return fmt.Errorf("build: refusing to delete filesystem root %q", abs)
	}
	if cwd, err := os.Getwd(); err == nil {
		if cwdAbs, err := filepath.Abs(cwd); err == nil {
			// rel goes "up" (starts with "..") only when cwd is OUTSIDE abs;
			// otherwise abs is the cwd or an ancestor of it -> unsafe to delete.
			if rel, err := filepath.Rel(abs, cwdAbs); err == nil {
				safe := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
				if !safe {
					return fmt.Errorf("build: refusing to delete %q (the current directory or an ancestor of it)", abs)
				}
			}
		}
	}
	return nil
}

func (a *App) writeRoute(outDir string, rt route, params map[string]string) error {
	url := fillPattern(rt, params)
	req := httptest.NewRequest("GET", url, nil)
	html, err := a.render(rt, params, req)
	// A loader returning ErrNotFound means "this page doesn't exist" — skip it
	// (matches the runtime 404 in server.go) instead of aborting the whole build.
	if errors.Is(err, ErrNotFound) {
		fmt.Printf("  skip %s (ErrNotFound)\n", url)
		return nil
	}
	if err != nil {
		return err
	}
	dst := filepath.Join(outDir, filepath.FromSlash(url), "index.html")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	fmt.Println("  ->", url)
	return os.WriteFile(dst, html, 0o644)
}

func (a *App) copyAssets(dst string) error {
	assetFS, err := fs.Sub(a.fsys, "assets")
	if err != nil {
		return nil
	}
	return fs.WalkDir(assetFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(assetFS, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, p)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
}
