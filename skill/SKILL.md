---
name: building-monobin-sites
description: Use when creating or editing a Monobin site or app — the Go single-binary SSR + islands framework. Covers adding routes/pages, server-side loaders, interactive Preact islands, Tailwind styling, and the monobin CLI.
---

# Building a site with Monobin

Monobin renders HTML on the server and ships JS only for interactive "islands". The whole app compiles to one Go binary. Go core is standard-library only.

## Add a page (routing)
File = route. Drop a template in `app/routes/`:
- `app/routes/index.html` → `/`
- `app/routes/about.html` → `/about`
- `app/routes/blog/index.html` → `/blog`
- `app/routes/blog/[slug].html` → `/blog/:slug` (dynamic; `[ ]` = param). Static routes beat dynamic.

Templates use Go `html/template` over a shared `layout.html`:
```html
{{ define "title" }}{{ .Data.Title }}{{ end }}
{{ define "content" }}<h1 class="text-2xl font-bold">{{ .Data.Title }}</h1>{{ end }}
```

## Add server data (loader)
Register a loader keyed by route pattern in `framework/loaders.go` → `registerLoaders`. Its return value is the template's `.Data`. This is where DB/Redis/API calls go (server-only):
```go
a.loaders["/blog/:slug"] = func(c *Ctx) (any, error) {
    p, ok := findPost(c.Params["slug"])
    if !ok {
        return nil, ErrNotFound // 404 at runtime, skipped by `monobin build`
    }
    return p, nil
}
// Dynamic routes also need StaticPaths to be pre-rendered by `monobin build`:
a.staticPaths["/blog/:slug"] = func() ([]map[string]string, error) {
    return []map[string]string{{"slug": "hello"}}, nil
}
```

## Add interactivity (island)
1. Write a Preact component in `islands/src/`:
```jsx
import { useState } from "preact/hooks";
export default function Counter({ start = 0 }) {
  const [n, setN] = useState(start);
  return <button onClick={() => setN(n + 1)} class="rounded bg-slate-900 px-3 py-1 text-white">{n}</button>;
}
```
2. **Register it** in `islands/src/entry.js` (required, else it won't hydrate):
```js
import Counter from "./counter.jsx";
const islands = { Counter };
```
3. Mount it in a template — pages with no island ship zero JS:
```html
{{ island "Counter" (dict "start" 3) }}
```

## Styling (Tailwind v4)
Use utility classes in templates. Keep `@source "../../app"` in `islands/src/style.css` so classes used only in `.html` aren't purged. Rebuild with `cd islands && npm run build`.

## CLI
- `monobin serve` — SSR server · `go run . dev` — dev (HMR + live reload, run from repo root)
- `monobin build [dir]` — static export · `monobin routes [--json]` — list routes
- `monobin check [--json]` — validate (unregistered islands, missing StaticPaths, bad loader keys). Run it after edits.

## Gotchas
- An `island "X"` not in `entry.js`'s `islands = {}` map silently fails — `monobin check` catches it.
- A Tailwind class used only in a Go template disappears in prod unless `@source "../../app"` is present.
