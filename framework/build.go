package framework

import (
	"fmt"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
)

// BuildStatic renders every route to static HTML under outDir. Dynamic routes
// are expanded via their registered StaticPaths. Output is a plain folder you
// can serve from anywhere (Caddy file_server, R2, a CDN).
func (a *App) BuildStatic(outDir string) error {
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	for _, rt := range a.routes {
		if rt.dynamic {
			sp, ok := a.staticPaths[rt.pattern]
			if !ok {
				fmt.Printf("  skip %-16s (dynamic, no StaticPaths registered)\n", rt.pattern)
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

func (a *App) writeRoute(outDir string, rt route, params map[string]string) error {
	url := fillPattern(rt, params)
	req := httptest.NewRequest("GET", url, nil)
	html, err := a.render(rt, params, req)
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
