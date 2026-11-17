package mjml

import (
	"flag"
	"strings"
	"testing"
)

// runKnownFailures makes TestMJMLKnownFailures fail, with the full difference
// report, instead of skipping:
//
//	go test ./mjml -run TestMJMLKnownFailures -known-failures
var runKnownFailures = flag.Bool("known-failures", false, "fail on known failures instead of skipping them, with the full difference report")

// knownFailures lists test cases of testdata, with an MJML 5.4.1 reference,
// that safe-mjml is known to render wrongly. They document bugs to fix rather
// than behavior to keep, so they are not in TestMJMLAgainstExpected.
//
// When a fix makes one of them match its reference, TestMJMLKnownFailures
// fails until the case is moved from this list to TestMJMLAgainstExpected.
var knownFailures = []struct {
	name   string
	since  string
	reason string
}{
	{
		name:   "mj-social-href-on-network",
		since:  "2026-11-14",
		reason: "on purpose (Pierre's decision): an href that already points to the network (https://x.com/notifuse, a facebook.com/sharer URL) is kept as written; MJML wraps every href in the network's share URL, which turns a profile link into a share link and doubles a share URL",
	},
	{
		name:   "mj-raw-white-space-pre",
		since:  "2026-11-12",
		reason: "an oracle artifact, not a safe-mjml gap: the reference is minified, and html-minifier (collapseWhitespace) collapses the spaces of a div with white-space:pre, which it does not read, so the reference displays differently from MJML's render; safe-mjml writes the mj-raw as MJML renders it, and the comparison keeps that whitespace, as a client displays it",
	},
	{
		name:   "mj-width-infinite",
		since:  "2026-11-12",
		reason: "a width too long a number for a float64 (400 digits) gives width=\"Infinity\" in MJML (parseInt); safe-mjml writes no Infinity width and keeps the value as written (Pierre's rule, as for NaN)",
	},
	{
		name:   "mj-ending-tag-unclosed",
		since:  "2026-11-12",
		reason: "an oracle artifact: the mj-raw opens an Outlook conditional comment that never ends, and html-minifier, which minifies the references, minifies its content as HTML and rebalances its tags; safe-mjml matches MJML's unminified render tag for tag (TestEndingTagWithoutClose)",
	},
	{
		name:   "mj-style-inline-cdata-unclosed",
		since:  "2026-11-13",
		reason: "an oracle artifact: with an inline mj-style, juice reads a CDATA section that never ends as a comment running to the end of the document, and cheerio closes the elements still open after it; html-minifier, which minifies the references, drops those close tags; safe-mjml matches MJML's unminified render (TestUnclosedCDATAWithInlineStyle)",
	},
	{
		name:   "mj-raw-unclosed-comment",
		since:  "2026-11-12",
		reason: "an oracle artifact: the mj-raw opens an Outlook conditional comment that never ends, and html-minifier, which minifies the references, minifies its content as HTML and rebalances its tags; safe-mjml matches MJML's unminified render tag for tag (TestEndingTagWithoutClose)",
	},
	{
		name:   "mj-navbar-base-url-no-href",
		since:  "2026-11-13",
		reason: "an mj-navbar-link without href in an mj-navbar with base-url: MJML writes the base-url followed by \"undefined\" (href=\"https://a.exampleundefined\"); safe-mjml writes the base-url alone, as for href=\"\" (Pierre's decision: no JavaScript artifact, as for NaN)",
	},
	{
		name:   "mj-hero-fluid-height-no-ratio",
		since:  "2026-11-12",
		reason: "an mj-hero in fluid-height mode without background-width and background-height: MJML writes padding-bottom:NaN% on the two cells around it, safe-mjml writes no padding-bottom, as it writes no NaN (Pierre's rule, as column-width-empty)",
	},
	{
		name:   "mj-section-padding-wider-than-body",
		since:  "2026-11-11",
		reason: "documented choice: safe-mjml writes no negative width, where MJML writes width:-50px. A section whose padding is wider than the body leaves its columns no room: MJML writes a 0px column and a -50px image (0 minus the image's padding), safe-mjml writes 0px and 0, since a computed width is never negative (SetContext) and a content never wider than its parent (mjml/testdata/AGENTS.md)",
	},
}

// TestMJMLKnownFailures renders the cases of knownFailures and checks that
// they still differ from their MJML 5.4.1 reference. Each one is skipped with
// its reason, so that `go test ./...` stays green while `go test -v` lists
// them; with -known-failures they fail with the full difference report.
// (P1, P2… refer to issues/architecture-foundations-refactor.md.)
func TestMJMLKnownFailures(t *testing.T) {
	for _, kf := range knownFailures {
		t.Run(kf.name, func(t *testing.T) {
			expected, actual, _, err := renderTestCase(t, kf.name)
			var diffs []string
			if err != nil {
				diffs = []string{"Render failed: " + err.Error()}
			} else {
				diffs = referenceDifferences(expected, actual)
			}

			if len(diffs) == 0 {
				t.Fatalf("%s now matches its MJML reference: move it from knownFailures to TestMJMLAgainstExpected", kf.name)
			}

			if *runKnownFailures {
				t.Errorf("known failure since %s: %s\n\n%s", kf.since, kf.reason, strings.Join(diffs, "\n\n"))
				return
			}
			t.Skipf("known failure since %s: %s (details: go test ./mjml -run 'TestMJMLKnownFailures/%s$' -known-failures)",
				kf.since, kf.reason, kf.name)
		})
	}
}
