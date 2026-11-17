package mjml

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/parser"
)

// randomIDs matches the 16-hex-digit IDs mj-carousel and mj-navbar draw at
// random on every render outside test mode.
var randomIDs = regexp.MustCompile(`[0-9a-f]{16}`)

// renderAllWays renders a document through every public entry point, and
// through the component tree the internal tests use. An entry point that
// fails is recorded as "error: <message>", so the entry points must also
// agree on failures.
func renderAllWays(t *testing.T, mjmlContent string) map[string]string {
	t.Helper()
	out := map[string]string{}
	record := func(entry, html string, err error) {
		if err != nil {
			html = "error: " + err.Error()
		}
		out[entry] = randomIDs.ReplaceAllString(html, "ID")
	}

	html, err := Render(mjmlContent)
	record("Render", html, err)

	result, err := Compile(mjmlContent)
	if result == nil {
		result = &Result{}
	}
	record("Compile", result.HTML, err)

	cache := NewCache(0)
	defer cache.Close()
	for _, entry := range []string{"Compile with a cache (miss)", "Compile with a cache (hit)"} {
		result, err := Compile(mjmlContent, WithCache(cache))
		if result == nil {
			result = &Result{}
		}
		record(entry, result.HTML, err)
	}

	doc, err := Parse(mjmlContent)
	if err != nil {
		record("Parse", "", err)
	} else {
		for _, entry := range []string{"Document.Compile", "Document.Compile again"} {
			result, err := doc.Compile()
			if result == nil {
				result = &Result{}
			}
			record(entry, result.HTML, err)
		}
		html, err := doc.Render()
		record("Document.Render", html, err)
	}

	ast, err := parser.ParseMJML(mjmlContent)
	if err != nil {
		t.Fatalf("ParseMJML: %v", err)
	}
	component, err := newComponentTree(ast)
	if err != nil {
		record("component tree", "", renderError(err))
	} else if root, ok := component.(*rootComponent); !ok || root.Body != nil {
		componentHTML, err := renderComponentString(component)
		if err != nil {
			err = renderError(err)
		}
		record("component tree", componentHTML, err)
	}
	return out
}

// TestPublicEntryPointsAgree checks that Render, Compile (with and without a
// cache) and Parse with Document.Compile (twice) and Document.Render produce
// the same HTML, or the
// same error, for every test document, as the component tree does.
func TestPublicEntryPointsAgree(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.mjml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			outputs := renderAllWays(t, string(content))
			want := outputs["Render"]
			for entry, got := range outputs {
				if got != want {
					t.Errorf("%s output differs from Render", entry)
				}
			}
		})
	}
}

// TestEntryPointsApplyHeadAttributes covers the case the entry points that
// rendered a parsed tree used to miss: attributes defined in mj-attributes.
func TestEntryPointsApplyHeadAttributes(t *testing.T) {
	const doc = `<mjml><mj-head><mj-attributes><mj-text color="#ff0000" /></mj-attributes></mj-head>` +
		`<mj-body><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body></mjml>`
	for entry, html := range renderAllWays(t, doc) {
		if !strings.Contains(html, "color:#ff0000") {
			t.Errorf("%s ignores mj-attributes: color #ff0000 missing", entry)
		}
	}
}

// chunkWriter is an io.StringWriter that is not a *strings.Builder, so
// rootComponent.Render renders into a builder of its own and copies it.
type chunkWriter struct{ out strings.Builder }

func (w *chunkWriter) WriteString(s string) (int, error) { return w.out.WriteString(s) }

// TestStreamingRenderMatchesBufferedRender checks that rendering a component
// tree to an arbitrary writer gives the same HTML as rendering it to a fresh
// strings.Builder.
func TestStreamingRenderMatchesBufferedRender(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.mjml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			ast, err := parser.ParseMJML(string(content))
			if err != nil {
				t.Fatalf("ParseMJML: %v", err)
			}
			render := func(w io.StringWriter) (string, error) {
				component, err := newComponentTree(ast, WithIDGenerator(sequentialTestIDs()))
				if err != nil {
					return "", err
				}
				return "", component.Render(w)
			}
			var buffered strings.Builder
			_, errBuffered := render(&buffered)
			var streamed chunkWriter
			_, errStreamed := render(&streamed)
			if (errBuffered == nil) != (errStreamed == nil) {
				t.Fatalf("errors differ: buffered %v, streamed %v", errBuffered, errStreamed)
			}
			if buffered.String() != streamed.out.String() {
				t.Errorf("streaming output differs from buffered output")
			}
		})
	}
}

// sequentialTestIDs returns a deterministic carousel and navbar ID generator.
func sequentialTestIDs() func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("%016x", n)
	}
}

// TestFragmentRenderMergesOutlookConditionals checks that a fragment root
// (here mj-body, without mjml) is refused by every entry point, as MJML
// throws "Malformed MJML" for it (checked with MJML 5.4.1 on this template).
// It checked, when fragments rendered, that they got the same Outlook merge
// as a document; Pierre decided that v1 follows MJML, and the merge of a
// document is compared with MJML on every test document.
func TestFragmentRenderMergesOutlookConditionals(t *testing.T) {
	fragment := `<mj-body><mj-section><mj-column><mj-text>A</mj-text></mj-column></mj-section>` +
		`<mj-wrapper full-width="full-width" background-url="https://example.com/bg.png"><mj-section><mj-column><mj-text>B</mj-text></mj-column></mj-section></mj-wrapper></mj-body>`
	for entry, html := range renderAllWays(t, fragment) {
		if !strings.HasPrefix(html, "error: ") || !strings.Contains(html, "the root element is mj-body, not mjml") {
			t.Errorf("%s: got %.80q, want the malformed MJML error", entry, html)
		}
	}
}
