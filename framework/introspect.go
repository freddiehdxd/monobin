package framework

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RouteInfo is the machine-readable shape of one route (for `monobin routes`).
type RouteInfo struct {
	Pattern        string `json:"pattern"`
	Template       string `json:"template"`
	Dynamic        bool   `json:"dynamic"`
	HasLoader      bool   `json:"hasLoader"`
	HasStaticPaths bool   `json:"hasStaticPaths"`
}

// RouteInfo returns every route with its flags, sorted as matched (static first).
func (a *App) RouteInfo() []RouteInfo {
	out := make([]RouteInfo, 0, len(a.routes))
	for _, rt := range a.routes {
		_, hasLoader := a.loaders[rt.pattern]
		_, hasStatic := a.staticPaths[rt.pattern]
		out = append(out, RouteInfo{
			Pattern:        rt.pattern,
			Template:       "app/" + rt.tmplName,
			Dynamic:        rt.dynamic,
			HasLoader:      hasLoader,
			HasStaticPaths: hasStatic,
		})
	}
	return out
}

// Finding is one result from Check. Level is "error" (fails `monobin check`) or
// "warn" (reported, non-fatal).
type Finding struct {
	Level   string `json:"level"`
	Where   string `json:"where"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
}

var (
	islandRefRe = regexp.MustCompile(`island\s+"([^"]+)"`)
	identRe     = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
)

// Check statically validates the app: templates parse, dynamic routes have
// StaticPaths (warn), islands referenced in templates are registered in
// entry.js, and loader/StaticPaths keys map to real routes.
func (a *App) Check() []Finding {
	out := []Finding{} // non-nil so an empty result marshals to [] (not null) for --json
	patterns := map[string]bool{}
	for _, rt := range a.routes {
		patterns[rt.pattern] = true
	}

	// 1. every route template parses (layout + page)
	for _, rt := range a.routes {
		if _, err := a.parse(rt.tmplName, &renderState{}); err != nil {
			out = append(out, Finding{"error", "app/" + rt.tmplName, err.Error(), "fix the template syntax"})
		}
	}

	// 2. dynamic routes should register StaticPaths (NoStatic routes are exempt)
	for _, rt := range a.routes {
		if rt.dynamic && !a.staticSkip[rt.pattern] {
			if _, ok := a.staticPaths[rt.pattern]; !ok {
				out = append(out, Finding{
					Level:   "warn",
					Where:   rt.pattern,
					Message: "dynamic route has no StaticPaths; 'monobin build' will skip it",
					Fix:     fmt.Sprintf("register a.staticPaths[%q] (or NoStatic it if SSR-only)", rt.pattern),
				})
			}
		}
	}

	// 3. islands referenced in templates must be registered in entry.js
	if reg, ok := islandRegistry(); ok {
		files := []string{"layout.html"}
		for _, rt := range a.routes {
			files = append(files, rt.tmplName)
		}
		for _, f := range files {
			for _, name := range a.islandRefs(f) {
				if !reg[name] {
					out = append(out, Finding{
						Level:   "error",
						Where:   "app/" + f,
						Message: fmt.Sprintf("references island %q but it is not registered in islands/src/entry.js", name),
						Fix:     fmt.Sprintf("add %s to the islands registry in islands/src/entry.js", name),
					})
				}
			}
		}
	} else {
		out = append(out, Finding{
			Level:   "warn",
			Where:   "islands/src/entry.js",
			Message: "could not read islands/src/entry.js — skipped the island-registration check",
			Fix:     "run `monobin check` from the repo root",
		})
	}

	// 4. loader / StaticPaths keys must map to a real route pattern
	for key := range a.loaders {
		if !patterns[key] {
			out = append(out, Finding{"error", key,
				"a loader is registered for a pattern with no matching route",
				"remove the loader or add a route file that maps to " + key})
		}
	}
	for key := range a.staticPaths {
		if !patterns[key] {
			out = append(out, Finding{"error", key,
				"StaticPaths is registered for a pattern with no matching route",
				"remove it or add a route file that maps to " + key})
		}
	}
	return out
}

func (a *App) islandRefs(tmplName string) []string {
	b, err := fs.ReadFile(a.fsys, tmplName)
	if err != nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, m := range islandRefRe.FindAllSubmatch(b, -1) {
		if n := string(m[1]); !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

// islandRegistry reads the component names from the `const islands = { ... }`
// object in islands/src/entry.js (relative to the working directory).
func islandRegistry() (map[string]bool, bool) {
	b, err := os.ReadFile(filepath.FromSlash("islands/src/entry.js"))
	if err != nil {
		return nil, false
	}
	m := regexp.MustCompile(`(?s)islands\s*=\s*\{(.*?)\}`).FindSubmatch(b)
	if m == nil {
		return map[string]bool{}, true
	}
	reg := map[string]bool{}
	for _, tok := range strings.Split(string(m[1]), ",") {
		if i := strings.IndexByte(tok, ':'); i >= 0 {
			tok = tok[:i] // "Name: Component" -> "Name"
		}
		if fields := strings.Fields(tok); len(fields) > 0 && identRe.MatchString(fields[0]) {
			reg[fields[0]] = true
		}
	}
	return reg, true
}
