package framework

import (
	"context"
	"net/http"
)

// Middleware wraps an http.Handler. This is the single extension hook the core
// owns; auth, logging, security headers, and rate limiting are all built on it
// in user-land — the framework ships none of them itself.
type Middleware func(http.Handler) http.Handler

// Use appends middleware to the chain applied around the route handler on the
// serve path. They run outermost-first in registration order: the first Use'd
// middleware sees the request first and the response last. Middleware never
// runs during `monobin build` (SSG renders routes directly).
func (a *App) Use(mw ...Middleware) {
	a.middleware = append(a.middleware, mw...)
}

// NoStatic marks route patterns the static builder must not pre-render (e.g.
// auth-gated or personalized SSR-only pages). They still render under `serve`.
func (a *App) NoStatic(patterns ...string) {
	for _, p := range patterns {
		a.staticSkip[p] = true
	}
}

type ctxKey int

const (
	patternKey ctxKey = iota
	matchKey
)

// matched carries the resolved route + params through the request context so
// the final handler can render without matching twice.
type matched struct {
	rt     route
	params map[string]string
}

// RoutePattern returns the matched route pattern (e.g. "/blog/:slug") for the
// current request, or "" if nothing matched. Middleware uses it to gate by
// pattern/prefix without re-parsing the URL.
func RoutePattern(r *http.Request) string {
	if p, ok := r.Context().Value(patternKey).(string); ok {
		return p
	}
	return ""
}

func withMatch(r *http.Request, m matched) *http.Request {
	ctx := context.WithValue(r.Context(), patternKey, m.rt.pattern)
	ctx = context.WithValue(ctx, matchKey, m)
	return r.WithContext(ctx)
}

func matchFrom(r *http.Request) (matched, bool) {
	m, ok := r.Context().Value(matchKey).(matched)
	return m, ok
}

// chain wraps h with the registered middleware, outermost-first.
func (a *App) chain(h http.Handler) http.Handler {
	for i := len(a.middleware) - 1; i >= 0; i-- {
		h = a.middleware[i](h)
	}
	return h
}
