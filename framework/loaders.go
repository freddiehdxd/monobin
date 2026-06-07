package framework

// Loader registers a server-side loader keyed by route pattern; its return value
// becomes the template's .Data. A loader returning ErrNotFound yields a 404 at
// runtime and is skipped by `monobin build`. This is where Postgres/Redis/API
// calls go — server-only, SEO-safe.
func (a *App) Loader(pattern string, fn Loader) {
	a.loaders[pattern] = fn
}

// StaticPaths registers the concrete param sets to pre-render for a dynamic
// route at build time (like Next/Astro getStaticPaths).
func (a *App) StaticPaths(pattern string, fn StaticPaths) {
	a.staticPaths[pattern] = fn
}
