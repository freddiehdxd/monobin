// Package auth is an EXAMPLE recipe — NOT part of the framework core. It shows
// that cookie-session auth is a few dozen lines of standard library in
// user-land, so the framework never has to own auth. Swap the in-memory store
// for Redis/Postgres in production.
//
// SSR-ONLY CAVEAT: auth applies to the `serve` (SSR) path only. Pages emitted by
// `monobin build` are public static files and cannot be gated. Mark gated routes
// with app.NoStatic(pattern) so the static builder skips them (see main.go).
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"github.com/freddiehdxd/monobin/framework"
)

const cookieName = "monobin_session"

// Store maps an opaque session id -> user id. In-memory; swap for Redis/Postgres.
type Store struct {
	mu       sync.Mutex
	sessions map[string]string
}

func NewStore() *Store { return &Store{sessions: map[string]string{}} }

func (s *Store) create(userID string) string {
	b := make([]byte, 16)
	rand.Read(b) // crypto/rand — never fails on the platforms we target
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[id] = userID
	s.mu.Unlock()
	return id
}

func (s *Store) user(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	uid, ok := s.sessions[c.Value]
	return uid, ok
}

func (s *Store) destroy(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// Middleware gates the `gate` route (and its subpaths) — redirecting signed-out
// visitors to /login — and owns the /login (POST) and /logout actions. It gates
// by matched route pattern (via framework.RoutePattern) plus path prefix.
func (s *Store) Middleware(gate string) framework.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/login" && r.Method == http.MethodPost:
				user := r.FormValue("user")
				if user == "" {
					user = "demo"
				}
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.create(user), Path: "/", HttpOnly: true})
				http.Redirect(w, r, gate, http.StatusFound)
				return
			case r.URL.Path == "/logout":
				if c, err := r.Cookie(cookieName); err == nil {
					s.destroy(c.Value)
				}
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			if framework.RoutePattern(r) == gate || strings.HasPrefix(r.URL.Path, gate+"/") {
				if _, ok := s.user(r); !ok {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
