package mjml

import (
	"fmt"
	stdhtml "html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/testutils"
)

// intendedRegressions lists the test documents (paths under testdata, without
// .mjml) on which the working tree deliberately renders further from MJML
// than the previous release, with the reason. The gate logs their
// regressions instead of failing, and fails when a listed document no longer
// regresses (a stale entry: remove it) or does not exist, as
// TestMJMLKnownFailures does for knownFailures.
//
// The list is for the release being prepared: once v1.0.0 is published it is
// the previous release, which already has these behaviours, so the list
// starts empty again.
var intendedRegressions = []struct {
	name   string
	since  string
	reason string
}{
	{
		name:   "mj-width-infinite",
		since:  "2026-11-12",
		reason: "Pierre's rule: safe-mjml writes no NaN or Infinity; v0.17.0 wrote width=\"Infinity\" as MJML does, the working tree writes the value as written",
	},
	{
		name:   "mj-style-inline-cdata-unclosed",
		since:  "2026-11-13",
		reason: "an oracle artifact, Pierre's decision: juice reads the CDATA section that never ends as a comment running to the end of the document and cheerio closes the elements after it, as MJML's unminified render does (TestUnclosedCDATAWithInlineStyle); the minified reference drops those close tags, so the render is further from it than v0.17.0's, which did not inline",
	},
}

// TestNoRegressionAgainstRelease is run by tools/release-diff.sh, which
// renders every test document with the previous release into
// SAFE_MJML_PREVIOUS_OUTPUT (same paths as in the repository, .html, plus a
// .failed file when the previous binary failed). Without it the test is
// skipped.
//
// Each document is cut into fragments (every tag with its attributes and
// effective style, every text, every CSS rule), inside the Outlook comments
// too. A fragment of the MJML reference that the previous release wrote and
// the working tree no longer writes is a regression, whether or not the
// document matched MJML as a whole: a document that already differed can
// still lose what it rendered right. The document, as a sequence of these
// fragments and of the conditional comment markers, must not get further from
// the reference either (tokenDistance): this catches what moves, into or out
// of an Outlook comment, or is added. A document that matched MJML must also
// still match it. A document the previous release failed to render must now
// match MJML, or be a known failure, whose reason says why it differs. The
// documents of intendedRegressions are logged instead of failing.
func TestNoRegressionAgainstRelease(t *testing.T) {
	previousDir := os.Getenv("SAFE_MJML_PREVIOUS_OUTPUT")
	if previousDir == "" {
		t.Skip("run by tools/release-diff.sh")
	}
	intended := map[string]string{}
	regressed := map[string]bool{}
	for _, r := range intendedRegressions {
		if _, err := os.Stat(filepath.Join("testdata", r.name+".mjml")); err != nil {
			t.Errorf("intendedRegressions lists %s, which does not exist: %v", r.name, err)
		}
		intended["testdata/"+r.name] = fmt.Sprintf("since %s: %s", r.since, r.reason)
	}
	// A known failure has been checked by hand: its reason says why it
	// differs from MJML.
	known := map[string]string{}
	for _, kf := range knownFailures {
		known["testdata/"+kf.name] = kf.reason
	}
	compared, fixed, other := 0, 0, 0
	for _, file := range releaseDocuments(t) {
		rel := strings.TrimSuffix(file, ".mjml")
		// regression reports a problem on rel: logged for an intended
		// regression, an error otherwise.
		regression := func(format string, args ...any) {
			if reason, ok := intended[rel]; ok {
				regressed[rel] = true
				t.Logf("intended regression (%s): %s", reason, fmt.Sprintf(format, args...))
				return
			}
			t.Errorf(format, args...)
		}
		reference, err := os.ReadFile(rel + ".html")
		if err != nil {
			continue // no MJML reference (e.g. a document MJML rejects)
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// The script writes the previous output under the repository paths
		// ("mjml" followed by the absolute path for the sweep documents).
		previousFile := filepath.Join(previousDir, "mjml", rel+".html")
		_, statErr := os.Stat(previousFile + ".failed")
		previousFailed := statErr == nil
		head, err := Render(string(source), WithIDGenerator(referenceIDGenerator(string(reference))))
		switch {
		case err != nil && previousFailed:
			continue // fails in both: an older gap, not a regression
		case err != nil:
			regression("REGRESSION %s: Render: %v (the previous release rendered it)", rel, err)
			continue
		case previousFailed && len(referenceDifferences(string(reference), head)) == 0:
			compared++
			fixed++ // the previous release failed on it, the working tree matches MJML
			continue
		case previousFailed && known[rel] != "":
			t.Logf("%s: the previous release failed to render it; the working tree differs from MJML, as knownFailures says: %s", rel, known[rel])
			continue
		case previousFailed:
			regression("%s: the previous release failed to render it, and the working tree differs from MJML (see %s.failed): check by hand", rel, previousFile)
			continue
		}
		previousBytes, err := os.ReadFile(previousFile)
		if err != nil {
			regression("%s: no previous output: %v", rel, err)
			continue
		}
		previous := withReferenceIDs(string(previousBytes), string(reference))
		if strings.TrimSpace(previous) == "" && strings.TrimSpace(head) != "" {
			regression("%s: the previous release rendered nothing: check by hand", rel)
			continue
		}
		compared++

		if previous != head {
			ref := documentTokens(string(reference))
			before, after := tokenDistance(ref, documentTokens(previous)), tokenDistance(ref, documentTokens(head))
			if after > before {
				regression("REGRESSION %s: further from the MJML reference than the previous release (%d tokens to change instead of %d): something was added, moved or lost", rel, after, before)
			}
		}
		if lost := lostFragments(string(reference), previous, head); len(lost) > 0 {
			regression("REGRESSION %s: the previous release wrote these fragments of the MJML reference, the working tree no longer does:\n  %s",
				rel, strings.Join(lost, "\n  "))
		}
		previousMatches := len(referenceDifferences(string(reference), previous)) == 0
		headDifferences := referenceDifferences(string(reference), head)
		switch {
		case previousMatches && len(headDifferences) > 0:
			regression("REGRESSION %s: matched MJML in the previous release, no longer does:\n%s", rel, strings.Join(headDifferences, "\n"))
		case !previousMatches && len(headDifferences) == 0:
			fixed++
		case !previousMatches:
			other++
		}
	}
	if compared == 0 {
		t.Fatalf("no previous output found in %s", previousDir)
	}
	for _, r := range intendedRegressions {
		if !regressed["testdata/"+r.name] {
			t.Errorf("intendedRegressions lists %s, which no longer regresses against the previous release: remove it", r.name)
		}
	}
	t.Logf("%d documents compared, %d fixed (now match MJML), %d still differing from MJML", compared, fixed, other)
}

// withReferenceIDs replaces the random IDs of a rendering made without
// WithIDGenerator (the previous release's CLI) by those of the reference, in
// order of first appearance, as referenceIDGenerator hands them out.
func withReferenceIDs(rendered, reference string) string {
	next := referenceIDGenerator(reference)
	ids := map[string]string{}
	return referenceRandomID.ReplaceAllStringFunc(rendered, func(match string) string {
		id := referenceRandomID.FindStringSubmatch(match)[1]
		if len(id) != 16 {
			return match
		}
		if _, ok := ids[id]; !ok {
			ids[id] = next()
		}
		return strings.TrimSuffix(match, id) + ids[id]
	})
}

var (
	tagParts     = regexp.MustCompile(`(?s)^<\s*(/?[A-Za-z][\w:-]*)(.*?)\s*/?>$`)
	tagAttribute = regexp.MustCompile(`(?s)([\w:-]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?`)
)

// lostFragments returns the fragments of reference that previous has more of
// than head (counted with multiplicity, up to the reference's count).
func lostFragments(reference, previous, head string) []string {
	ref, prev, cur := fragments(reference), fragments(previous), fragments(head)
	var lost []string
	for f, n := range ref {
		if min(n, prev[f]) > min(n, cur[f]) {
			lost = append(lost, f)
		}
	}
	sort.Strings(lost)
	return lost
}

// documentTokens cuts an HTML document into normalized tokens, in document
// order: start and end tags (attributes sorted, style reduced to its effective
// declarations), texts, CSS rules, and the markers of conditional comments,
// whose content is cut the same way.
func documentTokens(document string) []string {
	var tokens []string
	inStyle := false
	for _, token := range htmlTokens(document) {
		token = strings.TrimSpace(token)
		switch {
		case token == "":
			continue
		case token == "<!--" || token == "-->" || strings.HasPrefix(token, "<![endif]") || strings.HasPrefix(token, "[if "):
			tokens = append(tokens, "marker "+token)
		case strings.HasPrefix(token, "<"):
			tag := normalizeFragmentTag(token)
			tokens = append(tokens, tag)
			switch {
			case strings.HasPrefix(tag, "<style"):
				inStyle = true
			case tag == "</style>":
				inStyle = false
			}
		case inStyle:
			// One token per rule, whitespace collapsed (".a .b" is not
			// ".a.b"); an at-rule stays at the head of its first rule, so
			// that a rule moved out of it changes the sequence.
			for rule := range strings.SplitSeq(token, "}") {
				if rule = strings.Join(strings.Fields(rule), " "); rule != "" {
					tokens = append(tokens, "css "+rule+"}")
				}
			}
		default:
			// Entities decoded, so that &#38; equals &amp;; a non-breaking
			// space stays one, as it is not white space for the layout.
			text := strings.ReplaceAll(stdhtml.UnescapeString(token), "\u00a0", "&nbsp;")
			tokens = append(tokens, "text "+strings.Join(strings.Fields(text), " "))
		}
	}
	return tokens
}

// fragments counts the tokens of a document other than the markers, and the
// facts of its tags: each attribute, and each effective style declaration, of
// each tag name. A tag that already differs from the reference is never equal
// to it as a whole, so it is through its facts that losing another of its
// right attributes shows (an image whose src changes while its sizes was
// already missing).
func fragments(document string) map[string]int {
	counts := map[string]int{}
	for _, token := range documentTokens(document) {
		if !strings.HasPrefix(token, "marker ") {
			counts[token]++
		}
	}
	for _, token := range htmlTokens(document) {
		for _, fact := range tagFacts(strings.TrimSpace(token)) {
			counts[fact]++
		}
	}
	return counts
}

// tagFacts returns the facts of an opening tag: "fact <img src=…" for each
// attribute and "fact <td style padding:…" for each effective declaration.
func tagFacts(token string) []string {
	m := tagParts.FindStringSubmatch(token)
	if m == nil || strings.HasPrefix(m[1], "/") {
		return nil
	}
	name := strings.ToLower(m[1])
	var facts []string
	for _, a := range tagAttribute.FindAllStringSubmatch(m[2], -1) {
		key := strings.ToLower(a[1])
		value := stdhtml.UnescapeString(a[2] + a[3] + a[4])
		switch key {
		case "style":
			for prop, v := range testutils.EffectiveDeclarations(value) {
				if v != "" {
					facts = append(facts, fmt.Sprintf("fact <%s style %s:%s", name, prop, v))
				}
			}
		case "class":
			for _, class := range strings.Fields(value) {
				facts = append(facts, fmt.Sprintf("fact <%s class %s", name, class))
			}
		default:
			facts = append(facts, fmt.Sprintf("fact <%s %s=%q", name, key, value))
		}
	}
	return facts
}

// tokenDistance is the number of tokens to delete and insert to turn a into
// b (the length of both minus twice their longest common subsequence).
func tokenDistance(a, b []string) int {
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		a, b = a[1:], b[1:]
	}
	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a, b = a[:len(a)-1], b[:len(b)-1]
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else {
				cur[j] = max(prev[j], cur[j-1])
			}
		}
		prev, cur = cur, prev
	}
	return len(a) + len(b) - 2*prev[len(b)]
}

func normalizeFragmentTag(token string) string {
	m := tagParts.FindStringSubmatch(token)
	if m == nil {
		return strings.Join(strings.Fields(token), " ")
	}
	name := strings.ToLower(m[1])
	var attrs []string
	for _, a := range tagAttribute.FindAllStringSubmatch(m[2], -1) {
		key := strings.ToLower(a[1])
		value := stdhtml.UnescapeString(a[2] + a[3] + a[4])
		switch key {
		case "style":
			value = effectiveStyle(value)
		case "class":
			value = strings.Join(sortedFields(value), " ")
		}
		attrs = append(attrs, fmt.Sprintf("%s=%q", key, value))
	}
	sort.Strings(attrs)
	if len(attrs) == 0 {
		return "<" + name + ">"
	}
	return "<" + name + " " + strings.Join(attrs, " ") + ">"
}

func effectiveStyle(style string) string {
	var decls []string
	for prop, value := range testutils.EffectiveDeclarations(style) {
		if value != "" {
			decls = append(decls, prop+":"+value)
		}
	}
	sort.Strings(decls)
	return strings.Join(decls, ";")
}

func sortedFields(s string) []string {
	fields := strings.Fields(s)
	sort.Strings(fields)
	return fields
}

// releaseDocuments returns the test documents and, when tools/release-diff.sh
// runs with SWEEP=1, the generated layer sweep (SAFE_MJML_EXTRA_DOCUMENTS).
func releaseDocuments(t *testing.T) []string {
	t.Helper()
	files := invariantDocuments(t)
	if dir := os.Getenv("SAFE_MJML_EXTRA_DOCUMENTS"); dir != "" {
		extra, err := filepath.Glob(filepath.Join(dir, "*.mjml"))
		if err != nil || len(extra) == 0 {
			t.Fatalf("no document in SAFE_MJML_EXTRA_DOCUMENTS=%s: %v", dir, err)
		}
		files = append(files, extra...)
	}
	return files
}

// TestReleaseGateReadsWhatRenders checks the gate's fragments on the cases the
// reviews of the oracle found: equivalent serializations are equal, a right
// attribute lost in a tag that already differed is a loss.
func TestReleaseGateReadsWhatRenders(t *testing.T) {
	same := func(a, b string) bool {
		return tokenDistance(documentTokens(a), documentTokens(b)) == 0 && len(lostFragments(a, a, b)) == 0
	}
	equivalent := [][2]string{
		{`<p>Alpha &amp; Beta</p>`, `<p>Alpha &#38; Beta</p>`},
		{`<p>a &lt; b</p>`, `<p>a &#60; b</p>`},
		{`<td align="left" style="padding:1px;color:red">x</td>`, `<td style="color:red;padding:1px" align="left">x</td>`},
		{`<a href="?a=1&amp;b=2">x</a>`, `<a href="?a=1&#38;b=2">x</a>`},
	}
	for _, pair := range equivalent {
		if !same(pair[0], pair[1]) {
			t.Errorf("%q and %q render the same and must compare equal", pair[0], pair[1])
		}
	}
	different := [][2]string{
		{`<p>a&nbsp;b</p>`, `<p>a b</p>`},
		{`<style>.a .b{color:red}</style>`, `<style>.a.b{color:red}</style>`},
	}
	for _, pair := range different {
		if same(pair[0], pair[1]) {
			t.Errorf("%q and %q render differently and must not compare equal", pair[0], pair[1])
		}
	}

	// The image already lacks sizes in the previous release; losing its right
	// src as well must be reported.
	reference := `<img src="https://example.org/a.png" width="550" sizes="100vw">`
	previous := `<img src="https://example.org/a.png" width="550">`
	head := `<img src="https://example.org/wrong.png" width="550">`
	if lost := lostFragments(reference, previous, head); len(lost) == 0 {
		t.Errorf("a src lost in a tag that already differed is not reported")
	}
	// A '>' in a quoted value does not end the tag: the src changed by a
	// space after its "/" is a loss too.
	reference = `<img src="https://e.com/a/>b.png" width="550" sizes="100vw">`
	previous = `<img src="https://e.com/a/>b.png" width="550">`
	head = `<img src="https://e.com/a />b.png" width="550">`
	if lost := lostFragments(reference, previous, head); len(lost) == 0 {
		t.Errorf("a quoted value changed around a '>' is not reported")
	}
	reference = `<img src="https://example.org/a.png" width="550" sizes="100vw">`
	previous = `<img src="https://example.org/a.png" width="550">`
	// Fixing one attribute of a tag that differs loses nothing.
	fixed := `<img src="https://example.org/a.png" width="550" sizes="100vw">`
	if lost := lostFragments(reference, previous, fixed); len(lost) != 0 {
		t.Errorf("a fix is reported as a loss: %v", lost)
	}
}

// TestReferenceComparisonIgnoresVMLAttributeOrder: VML attributes in another
// order are the same element (a review found the comparison rejected them).
func TestReferenceComparisonIgnoresVMLAttributeOrder(t *testing.T) {
	doc := func(rect string) string {
		return `<!doctype html><html><head></head><body><div><!--[if mso | IE]>` + rect + `</v:rect><![endif]--><p>x</p></div></body></html>`
	}
	if d := referenceDifferences(doc(`<v:rect fill="true" stroke="false">`), doc(`<v:rect stroke="false" fill="true">`)); len(d) != 0 {
		t.Errorf("reordered VML attributes reported as different:\n%s", strings.Join(d, "\n"))
	}
	if d := referenceDifferences(doc(`<v:rect fill="true" stroke="false">`), doc(`<v:rect stroke="true" fill="true">`)); len(d) == 0 {
		t.Errorf("a changed VML attribute is not reported")
	}
}

// htmlTokens cuts an HTML document into comment markers ("<!--", "-->",
// "<![endif]"), tags and texts, in order. A tag runs to the '>' that closes
// it outside its quoted attribute values ("a/>b" in a src does not end it).
func htmlTokens(document string) []string {
	var tokens []string
	text := 0 // start of the pending text
	flush := func(end int) {
		if end > text {
			tokens = append(tokens, document[text:end])
		}
	}
	for i := 0; i < len(document); {
		switch {
		case strings.HasPrefix(document[i:], "<!--"):
			flush(i)
			tokens = append(tokens, "<!--")
			i += 4
			text = i
		case strings.HasPrefix(document[i:], "-->"):
			flush(i)
			tokens = append(tokens, "-->")
			i += 3
			text = i
		case strings.HasPrefix(document[i:], "<![endif]"):
			flush(i)
			end := i + len("<![endif]")
			if end < len(document) && document[end] == '>' {
				end++
			}
			tokens = append(tokens, document[i:end])
			i = end
			text = i
		case document[i] == '<' && i+1 < len(document) && (document[i+1] == '/' || isASCIILetter(document[i+1])):
			end := tagEnd(document, i)
			if end < 0 && document[i+1] == '/' && i+2 < len(document) && isASCIILetter(document[i+2]) {
				// A close tag that meets a "<" before its ">" (">Hi</mj-text</div>"):
				// an HTML parser reads it with what follows as one close tag,
				// which hides the next tag. It is a token of its own, so that
				// it differs from a document without it.
				if next := strings.IndexByte(document[i+2:], '<'); next >= 0 {
					flush(i)
					tokens = append(tokens, document[i:i+2+next])
					i += 2 + next
					text = i
					continue
				}
			}
			if end < 0 {
				i++
				continue
			}
			flush(i)
			tokens = append(tokens, document[i:end])
			i = end
			text = i
		default:
			i++
		}
	}
	flush(len(document))
	return tokens
}

// tagEnd returns the index after the '>' that ends the tag starting at i,
// outside quoted values, or -1.
func tagEnd(s string, i int) int {
	var quote byte
	for j := i + 1; j < len(s); j++ {
		c := s[j]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return j + 1
		case c == '<':
			return -1
		}
	}
	return -1
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
