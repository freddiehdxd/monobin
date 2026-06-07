package framework

import (
	"encoding/json"
	"os"
	"testing"
)

// TestCheckEmptyIsJSONArray locks the agent/CI-friendly contract: a clean app
// must marshal to "[]", never "null", so a JSON consumer can always iterate.
func TestCheckEmptyIsJSONArray(t *testing.T) {
	// Run from the repo root so islands/src/entry.js is readable and the island
	// check doesn't emit a partial-check warning.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	a := newAppFromFiles(t, false, map[string]string{
		"layout.html":       `<body>{{ block "content" . }}{{ end }}</body>`,
		"routes/index.html": `{{ define "content" }}clean{{ end }}`,
	})
	got := a.Check()
	if len(got) != 0 {
		t.Fatalf("expected a clean app to have no findings, got %d: %+v", len(got), got)
	}
	if b, _ := json.Marshal(got); string(b) != "[]" {
		t.Errorf(`json.Marshal(Check()) = %s, want "[]"`, b)
	}
}
