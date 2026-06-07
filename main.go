package main

import (
	"embed"
	"fmt"
	"os"

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

	dev := cmd == "dev"
	app, err := framework.New(appFS, dev)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

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
	default:
		fmt.Println("usage: monobin [serve|dev|build [outdir]]")
	}
}
