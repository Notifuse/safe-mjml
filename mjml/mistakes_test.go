package mjml

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestAuthorMistakes renders the documents of testdata/mistakes, realistic
// templates with one author mistake each (tools/mjml-reference/mistakes.mjs:
// a close tag forgotten or doubled, another case, a comment with "--", an
// attribute without a value or without quotes, text before <mjml>...), and
// checks that safe-mjml renders each one as MJML 5.4.1 does, or returns an
// error: never anything else, such as an email that silently lost the rest
// of its content.
//
// A refusal is allowed (safe-mjml reads the template as XML outside ending
// tags, and is stricter than MJML there), and logged. A render that differs
// from MJML fails, unless the document is listed in
// testdata/mistakes-known-failures.txt ("name: reason"); a listed document
// that matches MJML or is refused fails too, so that the list cannot go stale.
// The documents MJML cannot render minified (rejected.json) have no reference:
// they must not make safe-mjml panic, and safe-mjml must refuse those MJML throws
// "Malformed MJML" for (an element after </mjml>). A panic, which Render returns as an error
// whose cause is ErrInternal, is never an accepted refusal.
func TestAuthorMistakes(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "mistakes", "*.mjml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no document in testdata/mistakes (run node mistakes.mjs in tools/mjml-reference): %v", err)
	}
	sort.Strings(files)
	var rejected map[string]string
	data, err := os.ReadFile(filepath.Join("testdata", "mistakes", "rejected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &rejected); err != nil {
		t.Fatal(err)
	}
	known := readKnownFailuresFile(t, "mistakes-known-failures.txt")
	present := map[string]bool{}
	var refused []string
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		present[name] = true
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if message, ok := rejected[name]; ok {
			_, err := Render(string(source))
			switch {
			case errors.Is(err, ErrInternal):
				t.Errorf("%s: panic: %v", name, err)
			case err == nil && strings.HasPrefix(message, "Malformed MJML"):
				// MJML throws for the template itself, not html-minifier
				// for its output: an email rendered from it is one MJML
				// never sends.
				t.Errorf("%s: MJML 5.4.1 refuses it (%s), safe-mjml renders it", name, message)
			}
			continue
		}
		reference, err := os.ReadFile(strings.TrimSuffix(file, ".mjml") + ".html")
		if err != nil {
			t.Errorf("%s: no reference and not in rejected.json: %v", name, err)
			continue
		}
		html, renderErr := Render(string(source), WithIDGenerator(referenceIDGenerator(string(reference))))
		_, isKnown := known[name]
		if errors.Is(renderErr, ErrInternal) {
			// A panic returned as an error is never an accepted refusal.
			t.Errorf("%s: panic: %v", name, renderErr)
			continue
		}
		if renderErr != nil {
			refused = append(refused, fmt.Sprintf("%s: %v", name, strings.SplitN(renderErr.Error(), "\n", 3)[0:min(2, len(strings.SplitN(renderErr.Error(), "\n", 3)))]))
			if isKnown {
				t.Errorf("%s is refused (%v), but mistakes-known-failures.txt lists it as rendered differently: remove it", name, renderErr)
			}
			continue
		}
		differences := referenceDifferences(string(reference), html)
		switch {
		case isKnown && len(differences) == 0:
			t.Errorf("%s now matches MJML: remove it from testdata/mistakes-known-failures.txt", name)
		case !isKnown && len(differences) > 0:
			t.Errorf("%s renders without an error, but differently from MJML 5.4.1 (%s):\n%s", name, file, strings.Join(differences, "\n"))
		}
	}
	for name := range known {
		if !present[name] {
			t.Errorf("mistakes-known-failures.txt lists %s, which mistakes.mjs does not generate", name)
		}
	}
	t.Logf("%d documents: %d refused by safe-mjml, %d without an MJML reference", len(files), len(refused), len(rejected))
	for _, r := range refused {
		t.Logf("refused: %s", r)
	}
}
