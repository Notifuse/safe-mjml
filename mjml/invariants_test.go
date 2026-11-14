package mjml

import (
	"errors"
	"fmt"
	stdhtml "html"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/parser"
)

// Invariants every rendered document must satisfy, whatever MJML would write.
// They are checked on every test document and on the attribute matrix, and are
// the properties FuzzRender looks for. Each one guards a kind of regression
// found by reviews:
//   - the text of the document is visible outside the Outlook-only
//     conditional comments (content hidden in <!--[if mso | IE]> after a
//     misplaced section, an mj-raw, a full-width block);
//   - no pixel width is negative, not a finite number, or wider than the body
//     and every width attribute the source writes (boxes without room,
//     overflowing lengths), and no height is NaN or infinite;
//   - conditional comments open and close in turn, without nesting, and no
//     VML tag, opening or closing, is outside an Outlook conditional comment
//     ([if mso | IE], [if mso], [if gte mso 9]...), where other clients
//     would parse it;
//   - every word of the text of mj-body, read from the source itself and not
//     from the parsed tree, is visible: a parser that loses content (a close
//     tag swallowed, the rest of a document dropped) leaves it out of the
//     tree, where the visibility invariant cannot see it;
//   - two renders of the same document give the same HTML;
//   - a test document renders without error, and rendering never panics.

// textTags are the components whose content is text the reader must see.
var textTags = map[string]bool{
	"mj-text": true, "mj-button": true, "mj-navbar-link": true, "mj-social-element": true,
	"mj-accordion-title": true, "mj-accordion-text": true, "mj-table": true,
}

var (
	outlookOnlyBlock  = regexp.MustCompile(`(?s)<!--\[if [^!\]][^\]]*\]>.*?<!\[endif\]-->`)
	htmlComment       = regexp.MustCompile(`(?s)<!--.*?-->`)
	styleOrHead       = regexp.MustCompile(`(?is)<head>.*?</head>|<style[^>]*>.*?</style>`)
	anyTag            = regexp.MustCompile(`(?s)<[^>]*>`)
	word              = regexp.MustCompile(`[\p{L}\p{N}]{3,}`)
	pixelWidth        = regexp.MustCompile(`(?:^|[^-a-z])(?:max-)?width\s*(?::|=")\s*([-+]?[0-9]+(?:\.[0-9]+)?(?:e[-+]?[0-9]+)?)px`)
	widthAttribute    = regexp.MustCompile(`\swidth="([-+]?[0-9]+(?:\.[0-9]+)?(?:e[-+]?[0-9]+)?)(?:px)?"`)
	nonFiniteWidth    = regexp.MustCompile(`(?i)(?:^|[^-a-z])(?:min-|max-)?(?:width|height)\s*(?::|=")\s*[-+]?(?:nan|inf(?:inity)?)`)
	sourcePixels      = regexp.MustCompile(`width="([0-9]+(?:\.[0-9]+)?)px"`)
	vmlTag            = regexp.MustCompile(`</?v:[a-z]+`)
	conditionalMarker = regexp.MustCompile(`<!--\[if|<!\[endif\]`)
	// The delimiters of a downlevel-revealed conditional, whose content every
	// client shows: <!--[if mso]>--> and <!--<![endif]-->.
	revealedConditional = regexp.MustCompile(`<!--\[if [^\]]*\]>-->|<!--<!\[endif\]-->`)
)

// sourceWords returns the words of the text components of a document.
func sourceWords(ast *parser.MJMLNode) map[string]bool {
	words := map[string]bool{}
	var walk func(n *parser.MJMLNode)
	walk = func(n *parser.MJMLNode) {
		if textTags[n.GetTagName()] {
			text := stdhtml.UnescapeString(anyTag.ReplaceAllString(n.GetMixedContent()+" "+n.Text, " "))
			for _, w := range word.FindAllString(text, -1) {
				words[w] = true
			}
			return
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(ast)
	return words
}

var (
	sourceBody          = regexp.MustCompile(`(?is)<mj-body[\s>].*</mj-body\s*>`)
	sourceHiddenContent = regexp.MustCompile(`(?is)<!--.*?-->|<(style|script|mj-style)[\s>].*?</(style|script|mj-style)\s*>`)
)

// sourceTextWords returns the words of the text of mj-body, read from the
// source with regular expressions only, independently of the parser:
// everything between <mj-body> and its last close tag, without comments,
// <style> and <script>, and without the tags themselves (their attribute
// values included).
func sourceTextWords(source string) map[string]bool {
	words := map[string]bool{}
	body := sourceBody.FindString(source)
	body = sourceHiddenContent.ReplaceAllString(body, " ")
	text := stdhtml.UnescapeString(anyTag.ReplaceAllString(body, " "))
	for _, w := range word.FindAllString(text, -1) {
		words[w] = true
	}
	return words
}

// visibleWords returns the words a client that is not Outlook displays.
func visibleWords(html string) map[string]bool {
	text := styleOrHead.ReplaceAllString(html, " ")
	// <!--[if mso]>--> ends its own comment: what follows it is shown to
	// every client, up to <!--<![endif]-->.
	text = revealedConditional.ReplaceAllString(text, " ")
	text = outlookOnlyBlock.ReplaceAllString(text, " ")
	text = htmlComment.ReplaceAllString(text, " ")
	text = stdhtml.UnescapeString(anyTag.ReplaceAllString(text, " "))
	words := map[string]bool{}
	for _, w := range word.FindAllString(text, -1) {
		words[w] = true
	}
	return words
}

// bodyWidth returns the width of mj-body in pixels: its width attribute, else
// the one mj-attributes gives mj-body or mj-all, else 600.
func bodyWidth(ast *parser.MJMLNode) float64 {
	width := ""
	if body := ast.FindFirstChild("mj-body"); body != nil {
		width = body.GetAttribute("width")
	}
	if head := ast.FindFirstChild("mj-head"); width == "" && head != nil {
		for _, attributes := range head.Children {
			if attributes.GetTagName() != "mj-attributes" {
				continue
			}
			// mj-all reaches mj-body as it does in MJML; the mj-body tag wins.
			for _, tag := range []string{"mj-all", "mj-body"} {
				for _, def := range attributes.Children {
					if def.GetTagName() == tag && def.GetAttribute("width") != "" {
						width = def.GetAttribute("width")
					}
				}
			}
		}
	}
	if v, err := strconv.ParseFloat(strings.TrimSuffix(width, "px"), 64); err == nil {
		return v
	}
	return 600
}

// largestSourceWidth returns the largest pixel width attribute written in the
// source, which an element may legitimately be as wide as, even wider than the body
// (<mj-group width="401px"> in a 333px body, and the columns derived from it).
func largestSourceWidth(source string) float64 {
	largest := 0.0
	for _, m := range sourcePixels.FindAllStringSubmatch(source, -1) {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > largest {
			largest = v
		}
	}
	return largest
}

// checkInvariants returns the invariants a document breaks. A render error is
// one of them when reportErrors is set (test documents must render); the
// fuzzer leaves it out, as most mutated documents are invalid. reference is
// MJML 4.18's render of the document, or "" when there is none.
func checkInvariants(source, reference string, reportErrors bool) (problems []string) {
	defer func() {
		if r := recover(); r != nil {
			problems = append(problems, fmt.Sprintf("panic: %v\n%s", r, debug.Stack()))
		}
	}()
	ast, err := parser.ParseMJML(source)
	if err != nil {
		if reportErrors {
			problems = append(problems, fmt.Sprintf("render: parse error: %v", err))
		}
		return problems
	}
	// Render turns a panic into an error (ErrInternal): it is still a panic
	// here, for the test documents and the fuzzer alike.
	render := func() (string, error) {
		html, err := Render(source, WithIDGenerator(sequentialTestIDs()))
		var mjmlErr *Error
		if errors.Is(err, ErrInternal) && errors.As(err, &mjmlErr) {
			problems = append(problems, fmt.Sprintf("panic: %s\n%s", mjmlErr.Message, mjmlErr.Stack))
		}
		return html, err
	}
	first, err := render()
	if err != nil {
		if reportErrors && !errors.Is(err, ErrInternal) {
			problems = append(problems, fmt.Sprintf("render: %v", err))
		}
		return problems
	}
	if second, _ := render(); second != first {
		problems = append(problems, "determinism: two renders give different HTML")
	}
	// Conditional comments open and close in turn: Outlook does not nest them.
	depth := 0
	for _, marker := range conditionalMarker.FindAllString(first, -1) {
		if marker == "<!--[if" {
			depth++
		} else {
			depth--
		}
		if depth < 0 || depth > 1 {
			problems = append(problems, fmt.Sprintf("conditionals: a conditional comment is nested or closed twice (depth %d)", depth))
			break
		}
	}
	if depth > 0 {
		problems = append(problems, "conditionals: a conditional comment is never closed")
	}
	outsideOutlook := outlookOnlyBlock.ReplaceAllString(first, " ")
	if m := vmlTag.FindString(outsideOutlook); m != "" {
		problems = append(problems, fmt.Sprintf("vml: VML outside an Outlook conditional comment: %s", m))
	}
	visible := visibleWords(first)
	for w := range sourceWords(ast) {
		if !visible[w] {
			problems = append(problems, fmt.Sprintf("visibility: %q is not visible outside Outlook", w))
		}
	}
	// The fuzzer's mutated documents lose text as MJML does (an mj-text left
	// unclosed renders empty there too): the source is read only for the
	// test documents. A word MJML does not show either (an ending tag closed
	// in another case, an unknown element) is MJML's behaviour, not a loss.
	if reportErrors {
		var shownByMJML map[string]bool
		if reference != "" {
			shownByMJML = visibleWords(reference)
		}
		for w := range sourceTextWords(source) {
			if !visible[w] && (shownByMJML == nil || shownByMJML[w]) {
				problems = append(problems, fmt.Sprintf("content: %q, in the text of mj-body, is not visible outside Outlook", w))
			}
		}
	}
	// Layout widths only: the head holds media queries (max-width:479px).
	limit := max(bodyWidth(ast), largestSourceWidth(source))
	layout := styleOrHead.ReplaceAllString(first, "")
	for _, m := range nonFiniteWidth.FindAllString(layout, -1) {
		problems = append(problems, fmt.Sprintf("width: not a finite number: %s", m))
	}
	for _, re := range []*regexp.Regexp{pixelWidth, widthAttribute} {
		for _, m := range re.FindAllStringSubmatch(layout, -1) {
			v, err := strconv.ParseFloat(m[1], 64)
			if err == nil && (v < 0 || v > limit+0.001) {
				problems = append(problems, fmt.Sprintf("width: %vpx outside [0, %vpx]: %s", v, limit, strings.TrimSpace(m[0])))
			}
		}
	}
	return problems
}

// invariantExceptions lists, per document, the invariants it breaks on purpose
// or as a known failure, with the reason; its other invariants are checked.
// The invariants are the prefixes of the problems checkInvariants returns:
// panic, render, determinism, conditionals, vml, visibility, content, width.
var invariantExceptions = map[string]map[string]string{
	"mjml":                           {"render": "no mj-body: MJML 4.18 throws \"Malformed MJML\", so the render must fail"},
	"mj-raw-head":                    {"render": "no mj-body: MJML 4.18 throws \"Malformed MJML\", so the render must fail"},
	"mj-raw-unclosed-comment":        {"conditionals": "the mj-raw opens an Outlook conditional it never closes, which MJML writes as written"},
	"mj-ending-tag-unclosed":         {"conditionals": "the mj-raw opens an Outlook conditional it never closes, which MJML writes as written"},
	"mj-style-inline-cdata":          {"visibility": "with an inline mj-style, MJML writes a CDATA section as a comment (juice reads it with htmlparser2 in HTML mode), so its words are not shown, as in MJML (Pierre's decision)"},
	"mj-style-inline-cdata-unclosed": {"visibility": "with an inline mj-style, MJML writes a CDATA section as a comment, and one that never ends runs to the end of the document, so its words are not shown, as in MJML (the decision for mj-style-inline-cdata)"},
}

func invariantDocuments(t testing.TB) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"testdata/*.mjml", "testdata/matrix/*.mjml"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	return files
}

func TestRenderInvariants(t *testing.T) {
	for _, file := range invariantDocuments(t) {
		name := strings.TrimSuffix(file[len("testdata/"):], ".mjml")
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			excepted := map[string]bool{}
			reference, _ := os.ReadFile(strings.TrimSuffix(file, ".mjml") + ".html")
			for _, p := range checkInvariants(string(source), string(reference), true) {
				invariant, _, _ := strings.Cut(p, ":")
				if reason, ok := invariantExceptions[name][invariant]; ok {
					excepted[invariant] = true
					t.Logf("excepted (%s): %s", reason, p)
					continue
				}
				t.Error(p)
			}
			for invariant := range invariantExceptions[name] {
				if !excepted[invariant] {
					t.Errorf("invariantExceptions lists %s for %s, which keeps it now: remove the exception", invariant, name)
				}
			}
		})
	}
}

// FuzzRender mutates the test documents and checks that rendering never panics
// and always gives the same HTML (the other invariants depend on what the
// source asks for, which mutations make arbitrary). Run it with
// go test ./mjml -run '^$' -fuzz FuzzRender -fuzztime 1m.
func FuzzRender(f *testing.F) {
	for _, file := range invariantDocuments(f) {
		source, err := os.ReadFile(file)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(source))
	}
	f.Fuzz(func(t *testing.T, source string) {
		for _, p := range checkInvariants(source, "", false) {
			if strings.HasPrefix(p, "panic:") || strings.HasPrefix(p, "determinism:") {
				t.Fatal(p)
			}
		}
	})
}
