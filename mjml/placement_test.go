package mjml

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestPlacement renders the documents of testdata/placement, one for each
// pair of a body component and a container, allowed or not
// (tools/mjml-reference/placement.mjs), and checks that safe-mjml renders
// each one as MJML 5.4.1 does. MJML renders a misplaced component with a
// warning; an mj-group used to drop every child but its columns, and to
// write an Outlook cell around itself outside a section, and no document
// had either shape.
//
// Every document is valid XML, so a refusal fails as a difference does. A
// document listed in testdata/placement-known-failures.txt ("name: reason")
// must differ; one that matches MJML fails, so that the list cannot go
// stale. The documents MJML throws for (rejected.json: a misplaced child of
// mj-carousel) have no reference: they must not make safe-mjml panic.
func TestPlacement(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "placement", "*.mjml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no document in testdata/placement (run node placement.mjs in tools/mjml-reference): %v", err)
	}
	sort.Strings(files)
	var rejected map[string]string
	data, err := os.ReadFile(filepath.Join("testdata", "placement", "rejected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &rejected); err != nil {
		t.Fatal(err)
	}
	known := readKnownFailuresFile(t, "placement-known-failures.txt")
	present := map[string]bool{}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		present[name] = true
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := rejected[name]; ok {
			if _, err := Render(string(source)); errors.Is(err, ErrInternal) {
				t.Errorf("%s: panic: %v", name, err)
			}
			if _, isKnown := known[name]; isKnown {
				t.Errorf("%s has no MJML reference (rejected.json): remove it from placement-known-failures.txt", name)
			}
			continue
		}
		reference, err := os.ReadFile(strings.TrimSuffix(file, ".mjml") + ".html")
		if err != nil {
			t.Errorf("%s: no reference and not in rejected.json: %v", name, err)
			continue
		}
		html, renderErr := Render(string(source), WithIDGenerator(referenceIDGenerator(string(reference))))
		if renderErr != nil {
			t.Errorf("%s: refused, where MJML 5.4.1 renders it: %v", name, renderErr)
			continue
		}
		differences := referenceDifferences(string(reference), html)
		_, isKnown := known[name]
		switch {
		case isKnown && len(differences) == 0:
			t.Errorf("%s now matches MJML: remove it from testdata/placement-known-failures.txt", name)
		case !isKnown && len(differences) > 0:
			t.Errorf("%s renders differently from MJML 5.4.1 (%s):\n%s", name, file, strings.Join(differences, "\n"))
		}
	}
	for name := range known {
		if !present[name] {
			t.Errorf("placement-known-failures.txt lists %s, which placement.mjs does not generate", name)
		}
	}
}
