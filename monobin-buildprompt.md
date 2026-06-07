# Claude Code Build Prompt — `monobin` framework

> Paste everything below into Claude Code from the root of an empty repo.
> If a starter seed already exists in the repo, **reconcile/extend it — do not blindly overwrite**.

---

## Mission

Build **`monobin`** (brand: Monobin): a Go single-binary, HTML-first **islands** web framework. The entire app — routes, templates, and compiled client assets — ships as **one Go binary** you `scp` and run behind Caddy. No `node_modules` in production, no process manager for the app, no cloud lock-in.

One-line thesis: *"Astro's islands model, but as a single Go binary you can self-host."* SSR + SSG by default, JavaScript shipped only for the interactive bits.

## Hard constraints (read before writing any code)

- **Do NOT build a bundler, a renderer, or a reactivity system.** Those are Vite, Preact, and the browser. We own only conventions + glue. Reinventing any of them is out of scope and will be rejected.
- **Go side = standard library only.** Use `net/http`, `html/template`, `embed`, `io/fs`, `encoding/json`. **No web framework** (no Gin/Echo/Fiber/chi). If you think you need a Go dependency, STOP and ask first.
- **Client side = Vite + Preact + Tailwind v4.** Nothing else unless you ask.
- Keep the Go core small and readable (target: the whole framework package is a few hundred lines).
- Single binary is the product. Every decision serves "one file, zero ops."

## Stack & versions

- Go 1.23+
- Vite (current 5+), `@preact/preset-vite`, `preact@^10`
- `tailwindcss@4` + `@tailwindcss/vite@4` (pin exact patch, e.g. `4.3.x`, with `--save-exact`)

## Target layout

```
go.mod                      module github.com/<USER>/monobin   (set real path — see Phase 0)
main.go                     entrypoint: serve | dev | build [outdir]
  //go:embed all:app        templates + built assets baked into the binary
framework/
  app.go                    file-based router, dynamic-route matching, param extraction
  render.go                 SSR: layout + route + loader data + island helper
  server.go                 net/http server, /assets serving, dev live-reload (SSE)
  build.go                  SSG: render every route (expand dynamic via StaticPaths)
  loaders.go                user-defined server-side data + StaticPaths registry
  app_test.go               table tests for routing/matching (the tricky logic)
app/
  layout.html               root layout; links the stylesheet; injects island scripts
  routes/
    index.html              -> /
    about.html              -> /about
    blog/index.html         -> /blog          (list)
    blog/[slug].html        -> /blog/:slug     (dynamic, pre-rendered at build)
  assets/                   Vite build output (entry.js, style.css), embedded
islands/
  src/entry.js              hydration runtime: scans [data-island], mounts components
  src/style.css             @import "tailwindcss"; + @source for Go templates
  src/counter.jsx           example island
  vite.config.js            @tailwindcss/vite + preact; stable output filenames
  package.json
.github/workflows/ci.yml    go build + go vet + go test
LICENSE                     MIT
README.md
.gitignore
```

## Exact conventions (implement precisely)

**Routing.** Walk `app/routes/**/*.html`. `index.html` -> `/`, `about.html` -> `/about`, `blog/index.html` -> `/blog`, `blog/[slug].html` -> dynamic `/blog/:slug`. A path segment wrapped in `[ ]` is a dynamic param. Match static routes **before** dynamic ones (so `/blog/featured` beats `/blog/:slug`). Unknown path -> 404.

**Loaders.** Each route may register a server-side loader keyed by its pattern:
```go
type Ctx struct { Request *http.Request; Params map[string]string }
type Loader func(c *Ctx) (any, error)
```
The return value is the template's `.Data`. A loader returning `ErrNotFound` (a package sentinel) yields a 404 at runtime and is skipped at build.

**Dynamic SSG.** A dynamic route registers a `StaticPaths func() ([]map[string]string, error)` enumerating the param sets to pre-render (like Next/Astro `getStaticPaths`). `monobin build` expands these into one static HTML file per set.

**Template parsing gotcha.** Filenames like `[slug].html` contain glob metacharacters, so **do NOT use `template.ParseFS` / `ParseGlob`** on them. Read `layout.html` and the route file with `fs.ReadFile` and `template.Parse` instead.

**Island bridge.** A template helper emits an SSR placeholder; the client hydrates it.
- Template func `{{ island "Counter" (dict "start" 3) }}` outputs `<div data-island="Counter" data-props='{...json...}'></div>` (HTML-escape the JSON).
- `islands/src/entry.js`: query all `[data-island]`, look up the named component in a registry, JSON-parse `data-props`, `hydrate(h(Component, props), el)`. Components ship JS **only** for the islands a page actually uses; pages with no islands ship zero JS.

**Dev vs prod.**
- Dev (`monobin dev`): read `app/` from **disk** (edit + reload, no recompile); island scripts load from the Vite dev server (`http://localhost:5173`) for HMR; inject an SSE live-reload script; `/__live` endpoint polls template mtimes and pushes `reload`.
- Prod (`monobin serve`): serve everything from the **embedded** `app/` (`//go:embed all:app`); island scripts/styles load from `/assets/...`.

**Tailwind v4.**
- Add `@tailwindcss/vite` to `vite.config.js` plugins.
- `islands/src/style.css`: `@import "tailwindcss";` (NOT the old `@tailwind base/...` directives). Optional `@theme { ... }` for tokens.
- **Critical gotcha:** Tailwind auto-detection only scans files reachable from the Vite project. The Go templates in `app/**/*.html` are rendered outside Vite, so classes used only there get purged and silently disappear. Fix with an explicit source in the CSS: `@source "../../app";`.
- `import "./style.css"` in `entry.js` so it's part of the build.
- Configure Vite to emit **stable filenames** (`entry.js`, `style.css`) into `../app/assets` so Go can reference them without parsing a manifest. Link `<link rel="stylesheet" href="/assets/style.css">` in `layout.html` (prod). In dev, the Vite entry handles CSS.

**Embedding.** `//go:embed all:app` in `main.go`. Ensure `app/assets/` exists at compile time (commit a `.keep`) so the embed directive compiles before the first Vite build.

## Build in phases. After EACH phase, run the acceptance check and report pass/fail before continuing.

### Phase 0 — Repo setup
- Ask me for the GitHub path, set `module github.com/<USER>/monobin` in `go.mod`, and use that import path everywhere. If I don't answer, use `module monobin` and flag it for later rename.
- Create `.gitignore` (ignore `/dist/`, the built binary, `app/assets/*` except `.keep`, `node_modules/`).
- **Accept:** `go mod tidy` runs clean.

### Phase 1 — Go core (static routes, SSR, SSG, dev server)
Implement `app.go` (router + matching), `render.go` (layout+route+loader+island helper), `server.go` (http + /assets + SSE live-reload), `build.go` (SSG), `loaders.go` (a `/` loader). Static demo pages: home + about. `dict` + `island` template funcs.
- **Accept:** `go build` succeeds; `monobin serve` returns 200 for `/` and `/about`, 404 for unknown; `monobin build dist` writes `dist/index.html` and `dist/about/index.html`.

### Phase 2 — Dynamic routes + StaticPaths
Add `[param]` parsing, param extraction, static-before-dynamic ordering, `ErrNotFound` -> 404, `StaticPaths` expansion in the builder. Add a small in-memory `posts` dataset in `loaders.go` (this stands in for Postgres/Redis), a `/blog` list loader, a `/blog/:slug` detail loader, and its `StaticPaths`. Add `blog/index.html` and `blog/[slug].html`.
- **Accept:** `/blog` lists posts; `/blog/<known-slug>` renders that post and exposes `.Params.slug`; `/blog/<bad-slug>` -> 404; `monobin build dist` emits one `dist/blog/<slug>/index.html` per post.

### Phase 3 — Go tests
Write `app_test.go` table tests covering: static match, dynamic match + param capture, static-beats-dynamic precedence, no-match, root path, and `fillPattern` round-trips.
- **Accept:** `go test ./...` passes.

### Phase 4 — Islands (Vite + Preact)
Create `islands/` (package.json, vite.config.js, src/entry.js, src/counter.jsx). entry.js implements the registry + `[data-island]` hydration loop. Wire stable output filenames into `../app/assets`. Use the island in `routes/index.html`.
- **Accept:** `cd islands && npm install && npm run build` produces `app/assets/entry.js`; after `go build`, loading `/` in a browser shows the counter incrementing on click; `/about` ships no island markup. **Actually run the npm build and a browser/headless check — do not assume.**

### Phase 5 — Tailwind v4
Add `@tailwindcss/vite`, `src/style.css` with `@import "tailwindcss";` + `@source "../../app";`, import it in entry.js, emit `style.css` to assets, link it in `layout.html`. Restyle the demo pages with utility classes (clean, minimal, not a default-starter look).
- **Accept:** A Tailwind class used **only** in a Go `.html` template (e.g. a color on the `<h1>`) survives the production build and is visibly applied — proving `@source` works. Confirm by grepping the built `style.css` for that class.

### Phase 6 — Single-binary proof + OSS scaffolding
- Confirm the deploy story: `go build -o monobin .` then run `./monobin` and load every page **with the Vite dev server stopped** (assets come from the embedded build, not localhost:5173).
- Add `LICENSE` (MIT, placeholder for copyright holder), a real `README.md` (thesis, why-not-Next.js table, architecture, quickstart, honest "not for dashboards" scope, roadmap), and `.github/workflows/ci.yml` running `go build`, `go vet`, `go test` on push.
- **Accept:** CI workflow is valid; README quickstart commands match reality; the binary serves all pages standalone.

## Definition of done (report this checklist at the end)

- [ ] `go build` clean; `go vet` clean; `go test ./...` green
- [ ] `monobin dev`: edit a template -> browser auto-reloads; edit an island -> Vite HMR
- [ ] `monobin serve`: home, about, blog list, blog post all render server-side
- [ ] Dynamic route works at runtime (SSR) AND build time (one static file per slug)
- [ ] Bad slug -> 404
- [ ] Island hydrates and is interactive; no-island page ships zero JS
- [ ] Tailwind class used only in a Go template is NOT purged (`@source` proven)
- [ ] `./monobin` runs as a single binary with Vite stopped and (ideally) node absent
- [ ] LICENSE, README, CI present

## Reporting

At each phase, show me the exact commands you ran and their output for the acceptance check. If any dependency beyond the approved stack seems necessary, stop and ask. Keep the Go core dependency-free and small.
