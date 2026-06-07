# Monobin

**One binary. SSR + islands. Zero-ops deploy.**

Monobin is a tiny, HTML-first web framework for people who want Next.js-style
ergonomics without the deployment tax. You build server-rendered pages, sprinkle
in interactive "islands" only where you need them, and ship the *entire app —
templates, assets, and all — as a single Go binary*.

```
go build -o monobin .  # one file
scp monobin server:    # ship it
./monobin              # run it
```

No `node_modules` in production. No PM2 for the app itself. No standalone-output
gymnastics. Just a binary behind Caddy.

## Why it exists

Next.js is excellent, but self-hosting it means dragging the Node toolchain into
production. monobin's bet: most sites are 90% content and 10% interactivity. So
render HTML on the server (great SEO, instant first paint) and ship JavaScript
*only* for the interactive bits.

| | monobin | Next.js |
|---|---|---|
| Production artifact | one static binary | node + build output |
| Default output | HTML, JS opt-in | JS-heavy |
| Deploy | `scp` + run | runtime + process mgr |
| Interactivity | islands (Preact) | full React app |

**Honest scope:** this is "Astro-meets-Go," not a drop-in Next.js. It's built for
content-heavy sites — marketing, blogs, docs, programmatic SEO — **not** stateful
dashboards or app-shell SPAs. You trade React's ecosystem and RSC for radical
deployment simplicity. If you self-host and value one-binary ops, that's a great
trade. If you live in the React app world, it isn't.

## How it works

```
main.go                 entrypoint: serve | dev | build
  //go:embed all:app     <- templates + built assets baked into the binary
framework/
  app.go                 file-based router (app/routes/*.html -> URLs)
  render.go              SSR: layout + route + loader data + island mounts
  server.go              HTTP server, asset serving, dev live-reload (SSE)
  build.go               SSG: render every route to dist/
  loaders.go             your server-side data fetching, per route
app/
  layout.html            root layout
  routes/                file = route; [slug].html = dynamic route
  assets/                Vite build output, embedded at compile time
islands/                 the client side (Vite + Preact + Tailwind v4)
  src/entry.js           hydration runtime: finds [data-island], mounts components
  src/counter.jsx        example island
  src/style.css          @import "tailwindcss" + @source for the Go templates
  vite.config.js         stable output -> app/assets/{entry.js, style.css}
```

**Islands.** A template calls `{{ island "Counter" (dict "start" 3) }}`, which
emits `<div data-island="Counter" data-props="...">`. On the client, `entry.js`
finds those mounts and hydrates them with Preact. Pages with no islands ship no
JavaScript at all — the runtime `<script>` is emitted only when a page actually
renders an island.

**Styling.** Tailwind v4 via `@tailwindcss/vite`. Because the Go templates render
*outside* Vite, `src/style.css` adds `@source "../../app"` so utility classes used
only in `.html` files aren't purged. The compiled sheet is linked as
`/assets/style.css` in prod; in dev, Vite injects it with HMR.

**Loaders.** Each route can register a server-side `Loader` (see `loaders.go`)
that returns data exposed to the template as `.Data`. This is where Postgres /
Redis / API calls go — server-only, SEO-safe.

**Dev vs prod.** In dev, templates are read from disk (edit + reload, no
recompile) and island scripts load from the Vite dev server (HMR). In prod,
everything is served from the embedded copy in the binary.

## Quickstart

```bash
# 1. install island deps + build them once
cd islands && npm install && npm run build && cd ..

# 2a. production: single binary
go build -o monobin . && ./monobin            # http://localhost:3000

# 2b. development (two terminals)
cd islands && npm run dev               # island HMR on :5173
go run . dev                            # server on :3000, live reload

# 3. static export (optional)
go run . build dist                     # dist/ of plain HTML
```

## Roadmap (good first issues)

- [ ] Per-page island code-splitting (only load islands a page uses)
- [x] Dynamic routes (`routes/blog/[slug].html`) + StaticPaths for SSG ✓
- [ ] Nested layouts
- [ ] Streaming SSR
- [ ] `templ` option for typed components instead of `html/template`
- [ ] Middleware chain
- [ ] Built-in sitemap.xml + robots.txt generation

## License

MIT — make it yours.
