package mjml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unlistedDocuments are documents of testdata that are in neither
// expectedCases nor knownFailures, with the reason.
var unlistedDocuments = map[string]string{
	"mjml":        "an mjml element without mj-body: MJML 5.4.1 throws \"Malformed MJML\" (no reference can be generated); safe-mjml returns an *Error at the mjml element (TestNoBodyIsAnError)",
	"mj-raw-head": "an mj-head without mj-body, as mjml: MJML 5.4.1 throws \"Malformed MJML\"; safe-mjml returns an *Error",
}

// TestEveryDocumentIsListed checks that every document of testdata is
// compared with its reference (expectedCases), tracked as a known failure, or
// left out with a reason, and that every reference has its document: a
// document in none of the lists, or a reference alone, is never checked.
func TestEveryDocumentIsListed(t *testing.T) {
	listed := map[string]string{}
	for _, tc := range expectedCases {
		listed[tc.name] = "expectedCases"
	}
	for _, kf := range knownFailures {
		if where, ok := listed[kf.name]; ok {
			t.Errorf("%s is in both %s and knownFailures", kf.name, where)
		}
		listed[kf.name] = "knownFailures"
	}
	for name := range unlistedDocuments {
		if where, ok := listed[name]; ok {
			t.Errorf("%s is in both %s and unlistedDocuments", name, where)
		}
		listed[name] = "unlistedDocuments"
	}
	files, err := filepath.Glob(filepath.Join("testdata", "*.mjml"))
	if err != nil {
		t.Fatal(err)
	}
	// A reference without its document is never compared.
	references, err := filepath.Glob(filepath.Join("testdata", "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range references {
		if _, err := os.Stat(strings.TrimSuffix(reference, ".html") + ".mjml"); err != nil {
			t.Errorf("%s has no .mjml document: it is never compared", reference)
		}
	}
	present := map[string]bool{}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		present[name] = true
		if _, ok := listed[name]; !ok {
			t.Errorf("testdata/%s.mjml is in neither expectedCases, knownFailures nor unlistedDocuments", name)
		}
	}
	for name, where := range listed {
		if !present[name] {
			t.Errorf("%s lists %s, which has no testdata/%s.mjml", where, name, name)
		}
	}
}
