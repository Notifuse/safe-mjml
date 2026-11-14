package mjml

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// commentLine matches a line holding only an HTML comment: in the comment
// test documents, the comments between components are on lines of their own,
// and the one inside an mj-text is not.
var commentLine = regexp.MustCompile(`(?m)^[ \t]*<!--.*-->[ \t]*\n`)

// TestWithKeepCommentsFalse checks that WithKeepComments(false) renders the
// comment test documents as if their comments between components were not
// written, keeping the comments inside ending tags, as MJML's
// keepComments: false does. (Each document was also checked against MJML 4.18
// run with keepComments: false.)
func TestWithKeepCommentsFalse(t *testing.T) {
	for _, name := range []string{"comments-kept", "comments-kept-accordion", "comments-kept-navbar", "mj-body"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/" + name + ".mjml")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Render(string(source), WithKeepComments(false), WithIDGenerator(sequentialIDGenerator()))
			if err != nil {
				t.Fatal(err)
			}
			stripped := commentLine.ReplaceAllString(string(source), "")
			want, err := Render(stripped, WithIDGenerator(sequentialIDGenerator()))
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("WithKeepComments(false) differs from the document without its comments at byte %d", firstDifference(want, got))
			}
			if strings.Contains(string(source), "inside mj-text") && !strings.Contains(got, "<!-- inside mj-text, content -->") {
				t.Error("the comment inside mj-text must be kept: it is content")
			}
		})
	}
}

// TestCommentInCarousel checks that a comment inside an mj-carousel, where an
// mj-raw cannot go (MJML 4.18 fails on it), is dropped without a warning, and
// that the carousel still renders.
func TestCommentInCarousel(t *testing.T) {
	var warnings []Warning
	html, err := Render(`<mjml><mj-body><mj-section><mj-column><mj-carousel>
  <!-- in a carousel -->
  <mj-carousel-image src="https://example.com/1.png" />
  <mj-carousel-image src="https://example.com/2.png" />
</mj-carousel></mj-column></mj-section></mj-body></mjml>`, withWarningHandler(func(w Warning) { warnings = append(warnings, w) }))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: %v", warnings)
	}
	if strings.Contains(html, "in a carousel") {
		t.Error("the comment must be dropped")
	}
	if !strings.Contains(html, "https://example.com/2.png") {
		t.Error("the carousel images must be rendered")
	}
}
