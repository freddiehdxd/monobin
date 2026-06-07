package framework

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareOrderAndShortCircuit(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":         `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html":   `{{ define "content" }}HOME{{ end }}`,
		"routes/blocked.html": `{{ define "content" }}SECRET{{ end }}`,
	})

	var order []string
	a.Use(func(next http.Handler) http.Handler { // registered first -> outermost
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "A:before")
			next.ServeHTTP(w, r)
			order = append(order, "A:after")
		})
	})
	a.Use(func(next http.Handler) http.Handler { // gates /blocked by pattern
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if RoutePattern(r) == "/blocked" {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("blocked"))
				return // short-circuit: never calls next
			}
			order = append(order, "B")
			next.ServeHTTP(w, r)
		})
	})

	h := a.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if got := strings.Join(order, ","); got != "A:before,B,A:after" {
		t.Errorf("middleware order = %q, want A:before,B,A:after", got)
	}
	if !strings.Contains(rec.Body.String(), "HOME") {
		t.Errorf("expected render output, got %q", rec.Body.String())
	}

	order = nil
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest("GET", "/blocked", nil))
	if rec2.Code != http.StatusForbidden {
		t.Errorf("blocked status = %d, want 403", rec2.Code)
	}
	if strings.Contains(rec2.Body.String(), "SECRET") {
		t.Errorf("short-circuit failed — render leaked: %q", rec2.Body.String())
	}
	if got := strings.Join(order, ","); got != "A:before,A:after" {
		t.Errorf("blocked order = %q, want A:before,A:after (B returned before render)", got)
	}
}

func TestRoutePatternInContext(t *testing.T) {
	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":             `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/blog/[slug].html": `{{ define "content" }}x{{ end }}`,
	})
	var seen string
	a.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = RoutePattern(r)
			next.ServeHTTP(w, r)
		})
	})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/blog/hello", nil))
	if seen != "/blog/:slug" {
		t.Errorf("RoutePattern(r) = %q, want /blog/:slug", seen)
	}
}
