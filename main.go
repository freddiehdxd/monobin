package main

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/freddiehdxd/monobin/examples/auth"
	"github.com/freddiehdxd/monobin/framework"
)

// All app templates + built island assets are embedded into the binary.
// This is the entire deployment story: one file, no node_modules.
//
//go:embed all:app
var appFS embed.FS

// Starter project files emitted by `monobin new`.
//
//go:embed all:scaffold
var scaffoldFS embed.FS

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	// `new` scaffolds a fresh project and needs no app loaded.
	if cmd == "new" {
		if err := newProject(os.Args); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	// routes/check read app/ from disk (like dev) so they reflect current source.
	useDisk := cmd == "dev" || cmd == "routes" || cmd == "check"
	app, err := framework.New(appFS, useDisk)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// --- user-land wiring (the framework owns hooks, not policy) ---
	// This app's server-side data (loaders + StaticPaths) — see content.go.
	registerContent(app)

	// A trivial access log, registered first so it wraps every request.
	app.Use(accessLog())

	// Auth recipe (see examples/auth): cookie-session gate on /account, which
	// owns /login + /logout. NoStatic keeps the gated page out of `build` output
	// — auth is SSR-only; statically exported pages are public files.
	sessions := auth.NewStore()
	app.Use(sessions.Middleware("/account"))
	app.NoStatic("/account")

	// Demo: a redirect, a sitemap hint, and the origin for sitemap.xml/robots.txt.
	app.SiteURL = "https://example.com" // set to your real domain
	app.Redirect("/home", "/")
	app.Meta("/", map[string]string{"changefreq": "weekly", "priority": "1.0"})

	switch cmd {
	case "dev":
		// Dev: templates re-parsed per request, live-reload over SSE,
		// island scripts loaded from the Vite dev server (HMR).
		// Run `cd islands && npm run dev` in another terminal.
		fmt.Println("monobin dev  ->  http://localhost:3000   (run `cd islands && npm run dev` for island HMR)")
		if err := app.Serve(":3000"); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "build":
		// SSG: render every route to dist/ as static HTML.
		out := "dist"
		if len(os.Args) > 2 {
			out = os.Args[2]
		}
		if err := app.BuildStatic(out); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println("static site written to", out+"/")
	case "serve":
		fmt.Println("monobin  ->  http://localhost:3000")
		if err := app.Serve(":3000"); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "routes":
		// Introspection: print every route (human table, or --json for agents/CI).
		framework.PrintRoutes(os.Stdout, app.RouteInfo(), hasFlag("--json"))
	case "check":
		// Static validation; exits non-zero if any error-level finding.
		os.Exit(framework.PrintCheck(os.Stdout, app.Check(), hasFlag("--json")))
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		fmt.Fprintln(os.Stderr, "monobin: unknown command "+cmd)
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

const usage = "usage: monobin [serve|dev|build [outdir]|routes [--json]|check [--json]|new <dir>|help]"

func hasFlag(flag string) bool {
	for _, a := range os.Args[1:] {
		if a == flag {
			return true
		}
	}
	return false
}

var moduleNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// newProject writes the embedded scaffold into a fresh directory, filling in the
// module path (from the directory name) and pinning the framework version.
func newProject(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: monobin new <dir>")
	}
	dir := filepath.Clean(args[2])
	module := filepath.Base(dir)
	if !moduleNameRe.MatchString(module) {
		return fmt.Errorf("%q is not a valid module/dir name (use letters, digits, '.', '_', '-')", module)
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%q already exists", dir)
	}
	sub, err := fs.Sub(scaffoldFS, "scaffold")
	if err != nil {
		return err
	}
	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(sub, p)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(p, ".tmpl") // main.go.tmpl -> main.go
		if filepath.Base(name) == "gitignore" {
			name = name[:len(name)-len("gitignore")] + ".gitignore"
		}
		if name == "go.mod" {
			b = bytes.ReplaceAll(b, []byte("MODULE_NAME"), []byte(module))
			b = bytes.ReplaceAll(b, []byte("VERSION"), []byte(framework.Version))
		}
		out := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		return err
	}
	fmt.Printf("created %s/\n", dir)
	fmt.Printf("  next: cd %q && go mod tidy && (cd islands && npm install && npm run build) && go run .\n", dir)
	return nil
}

// accessLog is an example logging Middleware: it logs every request after the
// chain runs, including the matched route pattern (empty for unmatched paths).
func accessLog() framework.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			pat := framework.RoutePattern(r)
			if pat == "" {
				pat = "(no route)"
			}
			log.Printf("%s %s -> %s", r.Method, r.URL.Path, pat)
		})
	}
}
