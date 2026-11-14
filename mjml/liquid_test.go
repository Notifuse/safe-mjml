package mjml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	stdhtml "html"
)

// liquidMarkup finds the Liquid markup of a template: {{ }} outputs and
// {% %} tags.
var liquidMarkup = regexp.MustCompile(`(?s)\{\{.*?\}\}|\{%.*?%\}`)

// TestLiquidTemplates renders the documents of testdata/liquid, templates
// holding Liquid markup in attributes, in the content of ending tags, around
// components and in the head, each with and without an inline mj-style
// (tools/mjml-reference/liquid.mjs). safe-mjml renders templates before
// their Liquid is evaluated, so each must render as MJML 4.18 renders it,
// unless testdata/liquid-known-differences.txt lists it ("name: reason"); a
// listed document that matches MJML fails, so that the list cannot go stale.
//
// Whatever the comparison, safe-mjml must keep every markup of the template
// that MJML keeps (both outputs read with their entities decoded), and must
// render every document: these are valid templates. The documents MJML cannot
// render minified (rejected.json) have no reference: safe-mjml must render
// them and keep every markup of the template.
func TestLiquidTemplates(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "liquid", "*.mjml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no document in testdata/liquid (run node liquid.mjs in tools/mjml-reference): %v", err)
	}
	sort.Strings(files)
	var rejected map[string]string
	data, err := os.ReadFile(filepath.Join("testdata", "liquid", "rejected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &rejected); err != nil {
		t.Fatal(err)
	}
	known := readKnownFailuresFile(t, "liquid-known-differences.txt")
	present := map[string]bool{}
	matching := 0
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		present[name] = true
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var reference []byte
		if _, ok := rejected[name]; !ok {
			reference, err = os.ReadFile(strings.TrimSuffix(file, ".mjml") + ".html")
			if err != nil {
				t.Errorf("%s: no reference and not in rejected.json: %v", name, err)
				continue
			}
		}
		html, err := Render(string(source), WithIDGenerator(referenceIDGenerator(string(reference))))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		decoded := stdhtml.UnescapeString(html)
		for _, markup := range liquidMarkup.FindAllString(string(source), -1) {
			if strings.Contains(decoded, markup) {
				continue
			}
			if reference == nil || strings.Contains(stdhtml.UnescapeString(string(reference)), markup) {
				t.Errorf("%s: safe-mjml drops or changes %q, which MJML keeps", name, markup)
			}
		}
		if reference == nil {
			continue
		}
		_, isKnown := known[name]
		differences := referenceDifferences(string(reference), html)
		switch {
		case len(differences) == 0:
			matching++
			if isKnown {
				t.Errorf("%s now matches MJML: remove it from testdata/liquid-known-differences.txt", name)
			}
		case !isKnown:
			t.Errorf("%s renders differently from MJML 4.18 (%s):\n%s", name, file, strings.Join(differences, "\n"))
		}
	}
	for name := range known {
		if !present[name] {
			t.Errorf("liquid-known-differences.txt lists %s, which liquid.mjs does not generate", name)
		}
	}
	t.Logf("%d documents: %d identical to MJML, %d listed as different, %d without an MJML reference", len(files), matching, len(known), len(rejected))
}
