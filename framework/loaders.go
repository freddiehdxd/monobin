package framework

import (
	"time"
)

// --- demo data ---
// In a real app this is Postgres/Redis/an API. Kept in-memory here so the
// example runs with zero setup. This is the programmatic-SEO pattern: one
// dataset -> many server-rendered, near-zero-JS pages.

type Post struct {
	Slug, Title, Summary, Body string
}

var posts = []Post{
	{"hello-monobin", "Hello, monobin", "Why one binary beats a deploy pipeline.",
		"monobin renders this page on the server and ships no JavaScript for it."},
	{"islands-explained", "Islands, explained", "Ship JS only where it's needed.",
		"Static HTML everywhere; interactive widgets hydrate in isolation."},
	{"seo-at-scale", "SEO at scale", "Thousands of pages from one dataset.",
		"Each post here is a dynamic route pre-rendered at build time."},
}

func findPost(slug string) (Post, bool) {
	for _, p := range posts {
		if p.Slug == slug {
			return p, true
		}
	}
	return Post{}, false
}

// registerLoaders wires server-side data + static-path enumeration to routes.
func (a *App) registerLoaders() {
	a.loaders["/"] = func(c *Ctx) (any, error) {
		return map[string]any{
			"Title":   "Monobin",
			"Tagline": "One binary. SSR + islands. Zero-ops deploy.",
			"Year":    time.Now().Year(),
		}, nil
	}

	// /blog — list page
	a.loaders["/blog"] = func(c *Ctx) (any, error) {
		return posts, nil
	}

	// /blog/:slug — dynamic detail page
	a.loaders["/blog/:slug"] = func(c *Ctx) (any, error) {
		p, ok := findPost(c.Params["slug"])
		if !ok {
			return nil, ErrNotFound // -> 404 at runtime, skipped at build
		}
		return p, nil
	}

	// Tells `monobin build` which /blog/:slug pages to pre-render.
	a.staticPaths["/blog/:slug"] = func() ([]map[string]string, error) {
		out := make([]map[string]string, 0, len(posts))
		for _, p := range posts {
			out = append(out, map[string]string{"slug": p.Slug})
		}
		return out, nil
	}
}
