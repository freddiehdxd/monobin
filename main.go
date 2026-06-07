package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/freddiehdxd/monobin/examples/auth"
	"github.com/freddiehdxd/monobin/framework"
)

// All app templates + built island assets are embedded into the binary.
// This is the entire deployment story: one file, no node_modules.
//
//go:embed all:app
var appFS embed.FS

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	// routes/check read app/ from disk (like dev) so they reflect current source.
	useDisk := cmd == "dev" || cmd == "routes" || cmd == "check"
	app, err := framework.New(appFS, useDisk)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// --- user-land wiring (the framework owns hooks, not policy) ---
	// A trivial access log, registered first so it wraps every request.
	app.Use(accessLog())

	// Auth recipe (see examples/auth): cookie-session gate on /account, which
	// owns /login + /logout. NoStatic keeps the gated page out of `build` output
	// — auth is SSR-only; statically exported pages are public files.
	sessions := auth.NewStore()
	app.Use(sessions.Middleware("/account"))
	app.NoStatic("/account")

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
		printRoutes(app.RouteInfo(), hasFlag("--json"))
	case "check":
		// Static validation; exits non-zero if any error-level finding.
		os.Exit(runCheck(app.Check(), hasFlag("--json")))
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		fmt.Fprintln(os.Stderr, "monobin: unknown command "+cmd)
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

const usage = "usage: monobin [serve|dev|build [outdir]|routes [--json]|check [--json]|help]"

func hasFlag(flag string) bool {
	for _, a := range os.Args[1:] {
		if a == flag {
			return true
		}
	}
	return false
}

func printRoutes(infos []framework.RouteInfo, asJSON bool) {
	if asJSON {
		b, _ := json.MarshalIndent(infos, "", "  ")
		fmt.Println(string(b))
		return
	}
	yn := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PATTERN\tTEMPLATE\tDYNAMIC\tLOADER\tSTATICPATHS")
	for _, r := range infos {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Pattern, r.Template, yn(r.Dynamic), yn(r.HasLoader), yn(r.HasStaticPaths))
	}
	tw.Flush()
}

// runCheck prints findings and returns the process exit code (1 if any error).
func runCheck(findings []framework.Finding, asJSON bool) int {
	if asJSON {
		b, _ := json.MarshalIndent(findings, "", "  ")
		fmt.Println(string(b))
	}
	errs, warns := 0, 0
	for _, f := range findings {
		switch f.Level {
		case "error":
			errs++
		default:
			warns++
		}
		if !asJSON {
			fmt.Printf("%-5s %s\n        %s\n        fix: %s\n", strings.ToUpper(f.Level), f.Where, f.Message, f.Fix)
		}
	}
	if !asJSON {
		if errs == 0 && warns == 0 {
			fmt.Println("monobin check: OK — no problems found")
		} else {
			fmt.Printf("monobin check: %d error(s), %d warning(s)\n", errs, warns)
		}
	}
	if errs > 0 {
		return 1
	}
	return 0
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
