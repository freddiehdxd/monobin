package framework

import (
	"encoding/xml"
	"net/http"
	"strings"
)

// Redirect registers a permanent (301) redirect from one path to another. It is
// applied before route matching on the serve path, and emitted to a _redirects
// file (Netlify/Cloudflare format) by `monobin build`.
func (a *App) Redirect(from, to string) {
	a.redirects[from] = to
}

// Meta attaches arbitrary string metadata to a route pattern. It is exposed to
// the route's template as .Meta and used as sitemap hints (changefreq, priority).
func (a *App) Meta(pattern string, kv map[string]string) {
	a.meta[pattern] = kv
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// Sitemap renders sitemap.xml for every build-visible route (static routes plus
// dynamic routes expanded through their StaticPaths), with baseURL prepended.
// NoStatic routes are omitted, matching what `monobin build` emits.
func (a *App) Sitemap(baseURL string) ([]byte, error) {
	set := sitemapSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	add := func(pattern, path string) {
		u := sitemapURL{Loc: baseURL + path}
		if m := a.meta[pattern]; m != nil {
			u.ChangeFreq, u.Priority = m["changefreq"], m["priority"]
		}
		set.URLs = append(set.URLs, u)
	}
	for _, rt := range a.routes {
		if a.staticSkip[rt.pattern] {
			continue
		}
		if rt.dynamic {
			sp, ok := a.staticPaths[rt.pattern]
			if !ok {
				continue
			}
			sets, err := sp()
			if err != nil {
				continue // degrade by omission — one flaky route shouldn't sink the whole sitemap
			}
			for _, params := range sets {
				add(rt.pattern, fillPattern(rt, params))
			}
		} else {
			add(rt.pattern, fillPattern(rt, nil))
		}
	}
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

// Robots renders a robots.txt that allows everything and points at the sitemap.
func (a *App) Robots(baseURL string) []byte {
	return []byte("User-agent: *\nAllow: /\nSitemap: " + baseURL + "/sitemap.xml\n")
}

// baseURL is the configured SiteURL (trailing slash trimmed), or the origin
// derived from the request; for the build path (r == nil, no SiteURL) it returns
// an obvious placeholder rather than a deploy-looking localhost.
func (a *App) baseURL(r *http.Request) string {
	if a.SiteURL != "" {
		return strings.TrimRight(a.SiteURL, "/")
	}
	if r != nil {
		scheme, host := "http", "localhost"
		if r.TLS != nil {
			scheme = "https"
		}
		if r.Host != "" {
			host = r.Host
		}
		return scheme + "://" + host
	}
	return "https://example.com"
}

// render404 renders routes/404.html with a 404 status, or falls back to a plain
// Not Found when no custom page exists.
func (a *App) render404(w http.ResponseWriter, r *http.Request) {
	if a.notFound == "" {
		http.NotFound(w, r)
		return
	}
	html, err := a.render(route{pattern: "/404", tmplName: a.notFound}, nil, r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write(html)
}
