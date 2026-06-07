# My Monobin site

Generated with `monobin new`. SSR + islands, shipped as one Go binary.

## Quickstart

```bash
# 1. resolve the framework dependency
go mod tidy

# 2. install + build the island assets (once)
cd islands && npm install && npm run build && cd ..

# 3a. production: one binary
go build -o app-bin . && ./app-bin          # http://localhost:3000

# 3b. development (two terminals)
cd islands && npm run dev                    # island HMR on :5173
go run . dev                                 # server on :3000, live reload

# static export
go run . build dist
```

## Layout

```
main.go            entrypoint (//go:embed all:app)
app/
  layout.html      root layout
  routes/          file = route; [slug].html = dynamic route
  assets/          Vite output, embedded at compile time
islands/           Vite + Preact + Tailwind (the client side)
```

See `AGENTS.md` for conventions and the `monobin` framework docs for loaders,
middleware, and the `routes` / `check` introspection commands.
