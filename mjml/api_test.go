package mjml_test

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Notifuse/safe-mjml/mjml"
)

const helloTemplate = `<mjml><mj-body><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body></mjml>`

// asError returns err as an *mjml.Error, as a caller outside the package
// gets it, or fails the test.
func asError(t *testing.T, err error) *mjml.Error {
	t.Helper()
	var mjmlErr *mjml.Error
	if !errors.As(err, &mjmlErr) {
		t.Fatalf("expected an *mjml.Error, got %T: %v", err, err)
	}
	return mjmlErr
}

func TestParseErrorHasItsLine(t *testing.T) {
	const template = "<mjml>\n  <mj-body>\n    <mj-section>\n  </mj-body>\n</mjml>"
	for name, run := range map[string]func() error{
		"Render":  func() error { _, err := mjml.Render(template); return err },
		"Compile": func() error { _, err := mjml.Compile(template); return err },
		"Parse":   func() error { _, err := mjml.Parse(template); return err },
	} {
		t.Run(name, func(t *testing.T) {
			mjmlErr := asError(t, run())
			want := mjml.ErrorDetail{Line: 4, Message: "element <mj-section> closed by </mj-body>"}
			if len(mjmlErr.Details) != 1 || mjmlErr.Details[0] != want {
				t.Fatalf("Details = %+v, want [%+v]", mjmlErr.Details, want)
			}
			if got := mjmlErr.Error(); got != "mjml: the template is not valid XML:\n- Line 4 - element <mj-section> closed by </mj-body>" {
				t.Errorf("Error() = %q", got)
			}
		})
	}
}

func TestParseErrorWithoutLine(t *testing.T) {
	_, err := mjml.Render("")
	mjmlErr := asError(t, err)
	if len(mjmlErr.Details) != 0 || mjmlErr.Message != "mjml: the template has no root element" {
		t.Errorf("got %+v", mjmlErr)
	}
}

// TestRenderErrorHasItsTagAndLine checks that the error of a template MJML
// refuses (here an mjml without mj-body) has its tag and line, from every
// entry point. Parse refuses it too since Pierre decided that it fails
// early, so that a Document always renders: Parse is one of the entry points
// here, where the test used to render a Document parsed from this template.
func TestRenderErrorHasItsTagAndLine(t *testing.T) {
	const template = "\n\n<mjml lang=\"fr\">\n  <mj-head><mj-title>No body</mj-title></mj-head>\n</mjml>"
	for name, run := range map[string]func() error{
		"Render":  func() error { _, err := mjml.Render(template); return err },
		"Compile": func() error { _, err := mjml.Compile(template); return err },
		"Parse":   func() error { _, err := mjml.Parse(template); return err },
	} {
		t.Run(name, func(t *testing.T) {
			mjmlErr := asError(t, run())
			want := mjml.ErrorDetail{Line: 3, Message: "malformed MJML: the mjml element has no mj-body", TagName: "mjml"}
			if len(mjmlErr.Details) != 1 || mjmlErr.Details[0] != want {
				t.Fatalf("Details = %+v, want [%+v]", mjmlErr.Details, want)
			}
			if got := mjmlErr.Error(); got != "mjml: cannot render the template:\n- Line 3 of (mjml) - malformed MJML: the mjml element has no mj-body" {
				t.Errorf("Error() = %q", got)
			}
		})
	}
}

func TestUnknownRootIsAnErrorAtItsTag(t *testing.T) {
	_, err := mjml.Render("<mj-unknown></mj-unknown>")
	mjmlErr := asError(t, err)
	if len(mjmlErr.Details) != 1 || mjmlErr.Details[0].TagName != "mj-unknown" || mjmlErr.Details[0].Line != 1 {
		t.Errorf("Details = %+v", mjmlErr.Details)
	}
}

// An *mjml.Error built by the caller, as Notifuse builds them for its API,
// keeps its JSON names.
func TestErrorBuiltByTheCaller(t *testing.T) {
	err := error(&mjml.Error{Message: "template error", Details: []mjml.ErrorDetail{{Line: 2, Message: "bad", TagName: "mj-text"}}})
	if got := err.Error(); got != "template error:\n- Line 2 of (mj-text) - bad" {
		t.Errorf("Error() = %q", got)
	}
	if errors.Unwrap(err) != nil {
		t.Error("an Error built by the caller wraps nothing")
	}
	out, jsonErr := json.Marshal(err)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if string(out) != `{"message":"template error","details":[{"line":2,"message":"bad","tagName":"mj-text"}]}` {
		t.Errorf("JSON = %s", out)
	}
}

func TestCompileReturnsWarnings(t *testing.T) {
	const template = "<mjml>\n<mj-body>\n<mj-section><mj-column>\n<mj-text colour=\"red\">x</mj-text>\n<mj-bogus/>\n</mj-column></mj-section></mj-body></mjml>"
	res, err := mjml.Compile(template)
	if err != nil {
		t.Fatal(err)
	}
	want := []mjml.Warning{
		{Kind: mjml.WarningInvalidAttribute, TagName: "mj-text", Line: 4, Message: `Attribute colour is illegal`},
		{Kind: mjml.WarningUnknownTag, TagName: "mj-bogus", Line: 5, Message: `Element mj-bogus doesn't exist or is not registered`},
	}
	if len(res.Warnings) != len(want) {
		t.Fatalf("Warnings = %+v, want %+v", res.Warnings, want)
	}
	for i := range want {
		if res.Warnings[i] != want[i] {
			t.Errorf("Warnings[%d] = %+v, want %+v", i, res.Warnings[i], want[i])
		}
	}

	res, err = mjml.Compile(helloTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if res.Warnings == nil || len(res.Warnings) != 0 {
		t.Errorf("expected an empty, non-nil warning list, got %#v", res.Warnings)
	}
}

func TestDocumentRendersConcurrently(t *testing.T) {
	doc, err := mjml.Parse(helloTemplate)
	if err != nil {
		t.Fatal(err)
	}
	want, err := mjml.Render(helloTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			html, err := doc.Render()
			if err != nil {
				t.Errorf("Render: %v", err)
				return
			}
			if html != want {
				t.Error("Document.Render differs from Render")
			}
		}()
	}
	wg.Wait()
}

func TestCacheGivesTheSameOutput(t *testing.T) {
	cache := mjml.NewCache(0)
	want, err := mjml.Render(helloTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := mjml.Render(helloTemplate, mjml.WithCache(cache))
			if err != nil || got != want {
				t.Errorf("a render with the cache differs: %v", err)
			}
		}()
	}
	wg.Wait()
	cache.Close()
	cache.Close()
	if got, err := mjml.Render(helloTemplate, mjml.WithCache(cache)); err != nil || got != want {
		t.Errorf("a render with a closed cache differs: %v", err)
	}
	if got, err := mjml.Render(helloTemplate, mjml.WithCache(nil)); err != nil || got != want {
		t.Errorf("a render with a nil cache differs: %v", err)
	}
}

// The warnings found before a failure come with its *Error, and in its JSON.
func TestErrorCarriesTheWarningsBeforeIt(t *testing.T) {
	const template = "<mjml>\n<mj-head>\n<mj-attributes><mj-text colour=\"red\" /></mj-attributes>\n</mj-head>\n</mjml>"
	res, err := mjml.Compile(template)
	if res != nil {
		t.Fatalf("Compile returned a Result with its error: %+v", res)
	}
	mjmlErr := asError(t, err)
	want := mjml.Warning{Kind: mjml.WarningInvalidAttribute, TagName: "mj-text", Line: 3, Message: "Attribute colour is illegal"}
	if len(mjmlErr.Warnings) != 1 || mjmlErr.Warnings[0] != want {
		t.Fatalf("Warnings = %+v, want [%+v]", mjmlErr.Warnings, want)
	}
	out, jsonErr := json.Marshal(mjmlErr)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if !strings.Contains(string(out), `"warnings":[{"kind":"invalid-attribute","tagName":"mj-text","line":3,"message":"Attribute colour is illegal"}]`) {
		t.Errorf("JSON = %s", out)
	}
}

// A document without mj-body is an error, as MJML throws "Malformed MJML"
// for it.
func TestNoBodyIsAnError(t *testing.T) {
	for _, template := range []string{
		`<mjml lang="fr"></mjml>`,
		"<mjml>\n  <mj-head><mj-title>T</mj-title></mj-head>\n</mjml>",
	} {
		html, err := mjml.Render(template)
		if html != "" {
			t.Errorf("%q: Render returned HTML %q with its error", template, html)
		}
		mjmlErr := asError(t, err)
		want := mjml.ErrorDetail{Line: 1, Message: "malformed MJML: the mjml element has no mj-body", TagName: "mjml"}
		if len(mjmlErr.Details) != 1 || mjmlErr.Details[0] != want {
			t.Errorf("%q: Details = %+v, want [%+v]", template, mjmlErr.Details, want)
		}
	}
}

// Anything after the root element's close tag but whitespace and comments
// is a malformed MJML error at its line (Pierre's rule), from Parse too.
// Checked with MJML 4.18: it throws "Malformed MJML" for an element there
// (the first two templates), renders the second of two documents, and
// ignores text, close tags, processing instructions, declarations and CDATA
// sections there, which safe-mjml refuses on purpose.
func TestContentAfterTheRootIsAnError(t *testing.T) {
	const doc = "<mjml>\n<mj-body><mj-section><mj-column>\n<mj-text>Hello</mj-text>\n</mj-column></mj-section></mj-body>\n</mjml>"
	for _, c := range []struct {
		template string
		want     mjml.ErrorDetail
	}{
		{doc + "\n<mj-section><mj-column><mj-text>Pasted</mj-text></mj-column></mj-section>",
			mjml.ErrorDetail{Line: 6, Message: "malformed MJML: <mj-section> after the root element", TagName: "mj-section"}},
		{"<mjml><mj-body><mj-section><mj-column>\n<mj-text>Hello</mjml>\n<mj-button href=\"https://example.com\">Pay</mj-button>\n</mj-column></mj-section></mj-body></mjml>",
			mjml.ErrorDetail{Line: 3, Message: "malformed MJML: <mj-button> after the root element", TagName: "mj-button"}},
		{doc + "\n\n<MJML><mj-body></mj-body></MJML>",
			mjml.ErrorDetail{Line: 7, Message: "malformed MJML: <mjml> after the root element", TagName: "mjml"}},
		{doc + "<!-- a comment -->\nThanks!",
			mjml.ErrorDetail{Line: 6, Message: "malformed MJML: text after the root element"}},
		{doc + "\n</mj-body>", mjml.ErrorDetail{Line: 6, Message: "malformed MJML: </mj-body> after the root element", TagName: "mj-body"}},
		{doc + "<?php x ?>", mjml.ErrorDetail{Line: 5, Message: "malformed MJML: a processing instruction after the root element"}},
		{doc + "<!DOCTYPE html>", mjml.ErrorDetail{Line: 5, Message: "malformed MJML: a declaration after the root element"}},
		{doc + "<![CDATA[x]]>", mjml.ErrorDetail{Line: 5, Message: "malformed MJML: a CDATA section after the root element"}},
	} {
		html, err := mjml.Render(c.template)
		if html != "" {
			t.Errorf("%q: Render returned HTML with its error", c.template)
		}
		mjmlErr := asError(t, err)
		if mjmlErr.Message != "mjml: cannot render the template" || len(mjmlErr.Details) != 1 || mjmlErr.Details[0] != c.want {
			t.Errorf("%q: got %q %+v, want [%+v]", c.template, mjmlErr.Message, mjmlErr.Details, c.want)
		}
		out, jsonErr := json.Marshal(mjmlErr)
		if jsonErr != nil {
			t.Fatal(jsonErr)
		}
		wantJSON, _ := json.Marshal(c.want)
		if !strings.Contains(string(out), `"details":[`+string(wantJSON)+`]`) {
			t.Errorf("%q: JSON = %s", c.template, out)
		}
		if _, err := mjml.Parse(c.template); err == nil {
			t.Errorf("%q: Parse accepts it", c.template)
		}
	}
	// Whitespace and comments after the root are allowed, an unended
	// comment and the empty comments included.
	for _, after := range []string{"\n", " \t\r\n\f", "\n<!-- a comment -->\n", "<!--[if mso]><p>x</p><![endif]-->", "<!-->", "<!---->", "<!-- never ends"} {
		if _, err := mjml.Render(doc + after); err != nil {
			t.Errorf("%q after the root: %v", after, err)
		}
	}
}

// A comment before the root that never ends hides the rest of the template,
// as htmlparser2 reads it: MJML 4.18 fails ("Parsing failed. Check your
// mjml."), and safe-mjml returns an error, also when the comment holds no
// "<mjml" and the root follows it.
func TestUnendedCommentBeforeTheRootIsAnError(t *testing.T) {
	const doc = "<mjml><mj-body><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>"
	for _, template := range []string{
		"<!-- an old template, its comment never closed\n" + doc,
		"<!-- <mjml> in a comment never closed\n" + doc,
	} {
		if html, err := mjml.Render(template); err == nil {
			t.Errorf("%q: rendered, want an error:\n%s", template, html)
		}
	}
}

// Text before the mjml root is an XML syntax error at the line of its first
// character, not of the line breaks before it.
func TestTextBeforeTheRootHasItsLine(t *testing.T) {
	_, err := mjml.Render("\n\n  text<mjml></mjml>")
	mjmlErr := asError(t, err)
	if len(mjmlErr.Details) != 1 || mjmlErr.Details[0].Line != 3 {
		t.Errorf("Details = %+v, want line 3", mjmlErr.Details)
	}
}

// TestErrorDetailsAreNeverNull checks the JSON of the errors safe-mjml returns:
// details is a list, empty when there is no place to give, so that a
// consumer iterating it never gets null.
func TestErrorDetailsAreNeverNull(t *testing.T) {
	for _, template := range []string{"", "   ", "<!-- only a comment -->", "<mjml><mj-body><mj-section></mj-body></mjml>"} {
		_, err := mjml.Render(template)
		var mjmlErr *mjml.Error
		if !errors.As(err, &mjmlErr) {
			t.Fatalf("Render(%q): %v is not an *mjml.Error", template, err)
		}
		data, _ := json.Marshal(mjmlErr)
		if strings.Contains(string(data), `"details":null`) {
			t.Errorf("Render(%q): %s", template, data)
		}
	}
	_, err := mjml.Render("")
	if want := "mjml: the template has no root element"; err == nil || err.Error() != want {
		t.Errorf("empty template: %v, want %q", err, want)
	}
}

// A < that starts no well-formed tag, or a quote that opens no value, is
// raw content: MJML 4.18 renders these mj-raw as written. (The minified
// references cannot be generated: html-minifier refuses this HTML.)
func TestRawContentThatStartsNoTag(t *testing.T) {
	for _, raw := range []string{
		`<p>if a<b then 'x</p>`,
		`<p class=a title=it's>An apostrophe in an unquoted value</p>`,
	} {
		html, err := mjml.Render(`<mjml><mj-body><mj-raw>` + raw + `</mj-raw><mj-section><mj-column><mj-text>after</mj-text></mj-column></mj-section></mj-body></mjml>`)
		if err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if !strings.Contains(html, raw) || !strings.Contains(html, ">after<") {
			t.Errorf("%s: not written as is", raw)
		}
	}
}

// An ending tag MJML never sees closed, here because a comment in its
// content never ends, takes the rest of the document, as in
// mjml-parser-xml: its content is kept up to the last "</mj-raw", the
// elements after it are lost, and the open ones close. safe-mjml renders it as
// MJML's unminified render, tag for tag (the minified references differ by
// html-minifier's work inside the unended conditional comment).
func TestEndingTagWithoutClose(t *testing.T) {
	template, err := os.ReadFile("testdata/mj-ending-tag-unclosed.mjml")
	if err != nil {
		t.Fatal(err)
	}
	html, err := mjml.Render(string(template))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, ">kept<") || strings.Contains(html, "LOST") ||
		!strings.Contains(html, "</tr><!--[if mso | IE]></tbody>") {
		t.Errorf("not rendered as MJML renders it:\n%s", html)
	}
}

// MJML 4.18's juice throws (TypeError, reading 'indexOf') on a declaration
// without a name in a rule that matches an element, or in the style of an
// element a rule matches: a word before a semicolon, as the ";" of
// "&quot;" makes one. The render fails as MJML's does (Pierre's decision),
// and an element no rule matches keeps its style as written, as in MJML.
func TestInlineStyleUnnamedDeclarationFailsAsMJML(t *testing.T) {
	inText := func(css, html string) string {
		return `<mjml><mj-head><mj-style inline="inline">` + css + `</mj-style></mj-head>` +
			`<mj-body><mj-section><mj-column><mj-text>` + html + `</mj-text></mj-column></mj-section></mj-body></mjml>`
	}
	for name, template := range map[string]string{
		"a navbar link's font-family": `<mjml><mj-head><mj-style inline="inline">.x { color: red; }</mj-style></mj-head>` +
			`<mj-body><mj-section><mj-column><mj-navbar><mj-navbar-link css-class="x" href="https://example.com" font-family="&quot;Open Sans&quot;, Arial">Home</mj-navbar-link>` +
			`</mj-navbar></mj-column></mj-section></mj-body></mjml>`,
		"a style attribute": inText(".x { color: red; }", `<p class="x" style="font-family: &quot;Helvetica Neue&quot;, Arial; font-size: 14px;">x</p>`),
		"a rule":            inText(".x { color: red; bogus; }", `<p class="x">x</p>`),
	} {
		_, err := mjml.Render(template)
		if err == nil || !strings.Contains(err.Error(), "MJML fails on this template too") {
			t.Errorf("%s: got %v, want the error MJML gives", name, err)
		}
	}
	html, err := mjml.Render(inText(".x { color: red; bogus; }", `<p style="font-family: &quot;A&quot;">no rule matches it</p>`))
	if err != nil || !strings.Contains(html, `<p style="font-family: &quot;A&quot;">`) {
		t.Errorf("an element no rule matches: %v\n%s", err, html)
	}
	// juice names the declaration "undefined": a later "undefined" property
	// replaces it, and MJML renders (checked with MJML 4.18).
	for name, template := range map[string]string{
		"a later rule":  inText("p { bogus; } p { undefined: 1 }", "<p>x</p>"),
		"the own style": inText("p { bogus; }", `<p style="undefined:1">x</p>`),
	} {
		if html, err := mjml.Render(template); err != nil || !strings.Contains(html, `<p style="undefined: 1;">`) {
			t.Errorf("replaced by %s: %v", name, err)
		}
	}
}

// A style attribute that holds a template tag and a declaration juice cannot
// read fails in safe-mjml, which keeps the tag, where MJML's juice drops the
// whole attribute for its braces and renders: the error must not say that
// MJML fails too, and it shows the tags, not their placeholders.
func TestInlineStyleFailureWithTemplateTagsDoesNotBlameMJML(t *testing.T) {
	for _, style := range []string{
		`{% if a %}color:blue;{% endif %}font-family: &quot;Helvetica Neue&quot;, Arial`,
		`margin:0;{% a %}{`,
	} {
		_, err := mjml.Render(`<mjml><mj-head><mj-style inline="inline">.x { color: red; }</mj-style></mj-head>` +
			`<mj-body><mj-section><mj-column><mj-text><p class="x" style="` + style + `">t</p>` +
			`</mj-text></mj-column></mj-section></mj-body></mjml>`)
		switch {
		case err == nil:
			t.Errorf("%s: rendered, want juice's failure", style)
		case strings.Contains(err.Error(), "MJML fails"), strings.Contains(err.Error(), "juice_code_block"):
			t.Errorf("%s: %v", style, err)
		}
	}
}

// juice protects the template blocks {{ }} and <% %> from the HTML parser,
// and not Liquid's {% %}: in MJML, a Liquid tag inside an HTML tag loses its
// end (<p {% if x %}class="a" endif %}>). safe-mjml protects {% %} too, on
// purpose (Pierre's decision), so the tag comes out whole.
func TestInlineStyleKeepsLiquidTags(t *testing.T) {
	html, err := mjml.Render(`<mjml><mj-head><mj-style inline="inline">p { margin: 0; }</mj-style></mj-head>` +
		`<mj-body><mj-section><mj-column><mj-text><p {% if vip %}class="a"{% endif %}>Hello {{ name }}</p>` +
		`{% assign tag = "<p>" %}<p title="{% if x %}a{% else %}b{% endif %}">t</p></mj-text></mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`{% if vip %}class="a" {% endif %}`, `Hello {{ name }}`, `{% assign tag = "<p>" %}`, `title="{% if x %}a{% else %}b{% endif %}"`} {
		if !strings.Contains(html, want) {
			t.Errorf("%s is not kept:\n%s", want, html)
		}
	}
	// In the style attribute of an element a rule matches, a template block
	// alone between semicolons, or after the last one, is kept as written
	// (Pierre's decision, {{ }} included): juice drops it, or throws on it
	// before a semicolon, and a Liquid tag lost its end.
	for style, want := range map[string]string{
		"margin:0;{% if big %}font-size:20px;{% endif %}":                 `margin: 0; {% if big %}font-size: 20px; {% endif %};`,
		"{% if big %}font-size:20px;{% else %}font-size:12px;{% endif %}": `{% if big %}font-size: 20px; {% else %}font-size: 12px; {% endif %};`,
		"{% if a %}color:red;{% endif %}":                                 `{% if a %}color: red; {% endif %};`,
		"margin:0;{% if big %}font-size:20px;{% endif %};":                `margin: 0; {% if big %}font-size: 20px; {% endif %};`,
		"margin:0;{{ extra }}":                                            `margin: 0; {{ extra }};`,
		"margin:0;{{ custom }};padding:0":                                 `margin: 0; {{ custom }}; padding: 0;`,
		"color:{% if a %}red{% else %}blue{% endif %};margin:0":           `color: {% if a %}red{% else %}blue{% endif %}; margin: 0;`,
	} {
		html, err := mjml.Render(`<mjml><mj-head><mj-style inline="inline">.x { font-weight: bold; }</mj-style></mj-head>` +
			`<mj-body><mj-section><mj-column><mj-text><p class="x" style="` + style + `">t</p></mj-text></mj-column></mj-section></mj-body></mjml>`)
		if err != nil || !strings.Contains(html, `style="font-weight: bold; `+want+`"`) {
			t.Errorf("style=%q: %v, want %q in\n%s", style, err, want, html)
		}
	}
}

// With an inline mj-style, juice writes the close tags without a name, </>
// and </ >, as written, as MJML 4.18's unminified render shows (its minified
// references cannot have them: html-minifier refuses "</>").
func TestNamelessCloseTagsWithInlineStyle(t *testing.T) {
	html, err := mjml.Render(`<mjml><mj-head><mj-style inline="inline">.x { color: red; }</mj-style></mj-head>` +
		`<mj-body><mj-raw><p>A</>b</ >c</p>d</mj-raw></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `<p>A</>b</ >c</p>d`) {
		t.Errorf("not rendered as MJML renders it:\n%s", html)
	}
}

// With an inline mj-style, juice reads a style element as htmlparser2 8
// does: its text runs to "</style" followed by ">" or whitespace, and a
// <style/> opens a style whose content is markup, since HTML ignores the
// "/". MJML 4.18's unminified render keeps them so; its minified reference
// does not (html-minifier ends the style at "</style/>"), and the words of a
// <style/> are hidden, which the visibility invariant would report, so they
// are not in the MJML case mj-style-inline-raw-text-elements.
func TestStyleElementsWithInlineStyle(t *testing.T) {
	for _, raw := range []struct{ template, want string }{
		{`<style>p { }</style/><p class="a">not a close</p></style>`, `<style>p { }</style/><p class="a">not a close</p></style>`},
		{`<style/><p class="a">style</p></style>`, `<style><p class="a" style="color: red;">style</p></style>`},
	} {
		html, err := mjml.Render(`<mjml><mj-head><mj-style inline="inline">.a { color: red; }</mj-style></mj-head>` +
			`<mj-body><mj-raw>` + raw.template + `</mj-raw></mj-body></mjml>`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(html, `dir="auto">`+raw.want+`</div>`) {
			t.Errorf("%s: not rendered as MJML renders it:\n%s", raw.template, html)
		}
	}
}

// The pseudo-classes of form elements match with an inline mj-style, as in
// MJML 4.18, whose unminified render this is: its minified reference writes
// checked="checked" for checked (html-minifier), so it is not an MJML case.
func TestFormPseudoClassesWithInlineStyle(t *testing.T) {
	html, err := mjml.Render(`<mjml><mj-head><mj-style inline="inline">input:checked { color: red; } input:disabled { color: gray; } ` +
		`button:enabled { color: blue; } option:checked { color: purple; }</mj-style></mj-head><mj-body><mj-raw>` +
		`<input type="checkbox" checked> <input type="radio"> <input type="text" disabled>` +
		`<button>enabled</button> <button disabled>disabled</button>` +
		`<select><option selected>one</option><option>two</option></select></mj-raw></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	const want = `<input type="checkbox" checked style="color: red;"> <input type="radio"> <input type="text" disabled style="color: gray;">` +
		`<button style="color: blue;">enabled</button> <button disabled>disabled</button>` +
		`<select><option selected style="color: purple;">one</option><option>two</option></select>`
	if !strings.Contains(html, want) {
		t.Errorf("not rendered as MJML renders it:\n%s", html)
	}
}

// With an inline mj-style, mjml-core merges the Outlook conditionals once,
// after juice. A literal "<![endif]-->" in an mj-raw before a section leaves
// the section's Outlook table in a comment while juice runs, so the table is
// written as safe-mjml wrote it, its td unstyled: MJML 4.18's unminified render
// ends so (html-minifier refuses the stray "<![endif]-->", so there is no
// minified reference). safe-mjml merged before juice too, and juice styled the
// table's td.
func TestStrayEndifBeforeSectionWithInlineStyle(t *testing.T) {
	html, err := mjml.Render(`<mjml><mj-head><mj-style inline="inline">.a { color: red; } td { vertical-align: top; }</mj-style></mj-head>` +
		`<mj-body><mj-raw><![endif]--></mj-raw><mj-section><mj-column><mj-text><p class="a">one</p></mj-text></mj-column></mj-section>` +
		`<mj-section><mj-column><mj-text>two</mj-text></mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	// The first section's table, after the body's div (MJML's unminified
	// render has a line break between the two).
	const want = `dir="auto"><table align="center" border="0" cellpadding="0" cellspacing="0" class="" role="presentation" style="width:600px;" width="600" >` +
		`<tr><td style="line-height:0px;font-size:0px;mso-line-height-rule:exactly;"><![endif]-->`
	if !strings.Contains(html, want) {
		t.Errorf("not rendered as MJML renders it:\n%s", html)
	}
}

// With an inline mj-style, juice reads a CDATA section that never ends as a
// comment that runs to the end of the document, and cheerio writes the close
// tags of the elements still open after it. MJML 4.18's unminified render
// ends so (its minified reference drops the close tags, so the case is a
// known failure of the reference comparison).
func TestUnclosedCDATAWithInlineStyle(t *testing.T) {
	template, err := os.ReadFile("testdata/mj-style-inline-cdata-unclosed.mjml")
	if err != nil {
		t.Fatal(err)
	}
	html, err := mjml.Render(string(template))
	if err != nil {
		t.Fatal(err)
	}
	const mjmlEnd = `<![endif]--></div></body></html>]]--></div></td></tr></tbody></table></div></td></tr></tbody></table></div></div></body></html>`
	if !strings.Contains(html, `J<!--[CDATA[ left open</div>`) || !strings.HasSuffix(html, mjmlEnd) {
		t.Errorf("not rendered as MJML renders it:\n%s", html)
	}
}

// An ending tag left open is closed by the close tag of an element around
// it, as htmlparser2 closes it for MJML: its content is lost (there is no
// "</mj-text" before that close), and the rest of the document renders, on
// the lines of the template.
func TestEndingTagClosedByParent(t *testing.T) {
	result, err := mjml.Compile("<mjml><mj-body><mj-section><mj-column><mj-text>Hello\n<b>world</b>\n</mj-column></mj-section>\n" +
		"<mj-section><mj-column><mj-button foo=\"1\">Two</mj-button></mj-column></mj-section></mj-body></mjml>")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "world") || !strings.Contains(result.HTML, ">Two<") {
		t.Errorf("not rendered as MJML renders it:\n%s", result.HTML)
	}
	want := mjml.Warning{Kind: mjml.WarningInvalidAttribute, TagName: "mj-button", Line: 4, Message: "Attribute foo is illegal"}
	if len(result.Warnings) != 1 || result.Warnings[0] != want {
		t.Errorf("got %+v, want %+v", result.Warnings, want)
	}
}

// A close tag is read as htmlparser2 reads it after its "</". The spaces
// that follow "</" are skipped: </ mj-text> closes the mj-text, whose
// content is lost (there is no "</mj-text" in it), and the button and the
// mj-text after it are rendered; safe-mjml wrote their markup as the first
// mj-text's text (the MJML case mj-close-tag-space has the same mistakes in
// a template with spaces in container close tags too, which safe-mjml
// refused). Anything but a letter or '>' after "</" opens a comment up to
// the next '>': in </</mj-text>, </, shipped.</mj-text> or </-</mj-text>,
// the mj-text's close is in that comment, the mj-text runs to the column's
// close and keeps its content up to the last "</mj-text", the button's
// markup as text; safe-mjml closed the mj-text there and rendered the button.
// html-minifier refuses these renders, so they are compared with MJML
// 4.18's unminified ones.
func TestCloseTagAfterItsSlash(t *testing.T) {
	const after = "\n" + `<mj-button href="https://example.com">Pay</mj-button>` + "\n<mj-text>Bye</mj-text>\n"
	render := func(text string) string {
		t.Helper()
		html, err := mjml.Render("<mjml><mj-body><mj-section><mj-column>\n<mj-text>" + text + after +
			"</mj-column><mj-column><mj-text>B</mj-text></mj-column></mj-section></mj-body></mjml>")
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(html, `class="mj-column-per-50 mj-outlook-group-fix"`); n != 2 || !strings.Contains(html, ">B</div>") {
			t.Errorf("%s: %d columns, want 2 and B in the second:\n%s", text, n, html)
		}
		return html
	}
	html := render("Hello</ mj-text>")
	if strings.Contains(html, "Hello") || !strings.Contains(html, `color:#000000;"></div>`) ||
		!strings.Contains(html, ">Pay</a>") || !strings.Contains(html, ">Bye</div>") {
		t.Errorf("</ mj-text> is not the mj-text's close:\n%s", html)
	}
	for _, text := range []string{"Hello</</mj-text>", "Hello <b>Ada</, shipped.</mj-text>", "Hello</-</mj-text>"} {
		html := render(text)
		want := ">" + text + strings.TrimSuffix(after, "</mj-text>\n") + "</div>"
		if !strings.Contains(html, want) || strings.Contains(html, ">Pay</a>") {
			t.Errorf("%s: want %q, and no button, as in MJML:\n%s", text, want, html)
		}
	}
}

// htmlparser2's spaces (space, tab, CR, LF, FF) end a close tag's name,
// and are skipped after its "</", as MJML 4.18 renders these templates:
// </mj-text\t> closes the mj-text with its content, </\tmj-text> closes it
// empty (there is no "</mj-text" in it), and the mj-text after it renders.
func TestCloseTagNameAndSpaces(t *testing.T) {
	for _, space := range []string{" ", "\t", "\r\n", "\n", "\f"} {
		for close, want := range map[string]string{"</mj-text" + space + ">": ">A</div>", "</" + space + "mj-text>": `color:#000000;"></div>`} {
			html, err := mjml.Render("<mjml><mj-body><mj-section><mj-column><mj-text>A" + close +
				"<mj-text>B</mj-text></mj-column></mj-section></mj-body></mjml>")
			if err != nil {
				t.Fatalf("%q: %v", close, err)
			}
			if !strings.Contains(html, want) || !strings.Contains(html, ">B</div>") {
				t.Errorf("%q: want %s and >B</div> in:\n%s", close, want, html)
			}
		}
	}
}

// A "<" followed by a letter in an ending tag's content opens a tag for
// htmlparser2, which reads <cost</mj-title> as a tag named "cost<": the
// ending tag is then closed by its parent's close, as MJML 4.18 renders it.
// Its references cannot be minified (html-minifier refuses <cost</title>),
// so the render is compared here with MJML's unminified one: the title keeps
// "Sale: price<cost", the mj-preview after it is inside the title and lost,
// a "<" before a digit or a space is text, and the button closed by
// </mj-column> takes the mj-text after it.
func TestLessThanInEndingTagContent(t *testing.T) {
	html, err := mjml.Render("<mjml>\n<mj-head>\n<mj-title>Sale: price<cost</mj-title>\n</mj-head>\n<mj-body>\n" +
		"<mj-section><mj-column><mj-text>Body text</mj-text></mj-column></mj-section>\n</mj-body>\n</mjml>")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<title>Sale: price<cost</title>") || !strings.Contains(html, ">Body text<") {
		t.Errorf("not rendered as MJML renders it:\n%s", html)
	}

	html, err = mjml.Render("<mjml><mj-head><mj-title>Sale: price<cost</mj-title><mj-preview>Save 10<20%</mj-preview></mj-head>" +
		"<mj-body><mj-section><mj-column><mj-text>1<2 and 3 < 4</mj-text>" +
		`<mj-button href="https://example.com">a<b</mj-button><mj-text>After</mj-text>` +
		"</mj-column></mj-section></mj-body></mjml>")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>Sale: price<cost</title>", ">1<2 and 3 < 4<", ">a<b</a>"} {
		if !strings.Contains(html, want) {
			t.Errorf("want %s in:\n%s", want, html)
		}
	}
	for _, lost := range []string{"Save 10", "After"} {
		if strings.Contains(html, lost) {
			t.Errorf("%s is kept, MJML loses it:\n%s", lost, html)
		}
	}
}

// "<!-->" and "<!--->" are whole comments for htmlparser2, empty ones: in an
// ending tag's content they do not hide the rest of the document, which
// read them as comments that never end and wrote it as the mj-text's text.
// html-minifier rewrites them in a minified reference, so the render is
// compared here with MJML 4.18's unminified one: the mj-text and the mj-raw
// keep them as written, and the button and the second column are rendered.
func TestShortCommentInEndingTagContent(t *testing.T) {
	html, err := mjml.Render(`<mjml><mj-body><mj-section><mj-column><mj-text>Hello<!-->there</mj-text>` +
		`<mj-button href="https://example.com">Pay</mj-button></mj-column><mj-column>` +
		`<mj-raw><!---><p>raw</p></mj-raw><mj-text>Second column</mj-text></mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{">Hello<!-->there</div>", ">Pay</a>", "<!---><p>raw</p>", ">Second column</div>"} {
		if !strings.Contains(html, want) {
			t.Errorf("want %s in:\n%s", want, html)
		}
	}
	if n := strings.Count(html, `class="mj-column-per-50 mj-outlook-group-fix"`); n != 2 {
		t.Errorf("%d columns, want 2:\n%s", n, html)
	}
}

// An mj-navbar-link without href, in an mj-navbar with base-url, links to
// the base-url alone, as with href="": MJML writes the base-url followed by
// "undefined", a JavaScript artifact safe-mjml does not write (Pierre's
// decision, as for NaN; the MJML case is the known failure
// mj-navbar-base-url-no-href).
func TestNavbarLinkWithoutHrefGetsTheBaseURL(t *testing.T) {
	template, err := os.ReadFile("testdata/mj-navbar-base-url-no-href.mjml")
	if err != nil {
		t.Fatal(err)
	}
	html, err := mjml.Render(string(template))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `href="https://a.example"`) || strings.Contains(html, "undefined") {
		t.Errorf("want the base-url alone as href:\n%s", html)
	}
}

// A close tag's name runs to a space or '>', as htmlparser2 reads it:
// </mj-text/> is the close of an element "mj-text/", which is not open, and
// closes nothing. The mj-text then runs to its parent's close and keeps,
// up to its last "</mj-text", what MJML 4.18 keeps (compared with its
// unminified render; html-minifier rebalances this markup in a reference).
func TestCloseTagNameWithSlash(t *testing.T) {
	html, err := mjml.Render(`<mjml><mj-body><mj-section><mj-column><mj-text>A</mj-text/><mj-text>B</mj-text></mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `>A</mj-text/><mj-text>B</div>`) {
		t.Errorf("not rendered as MJML renders it:\n%s", html)
	}
}

// mj-wrapper's attributes are checked like those of the other components,
// as MJML checks them.
func TestWrapperAttributesAreValidated(t *testing.T) {
	result, err := mjml.Compile("<mjml><mj-body>\n<mj-wrapper foo=\"1\"><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-wrapper>\n</mj-body></mjml>")
	if err != nil {
		t.Fatal(err)
	}
	want := mjml.Warning{Kind: mjml.WarningInvalidAttribute, TagName: "mj-wrapper", Line: 2, Message: "Attribute foo is illegal"}
	if len(result.Warnings) != 1 || result.Warnings[0] != want {
		t.Errorf("got %+v, want %+v", result.Warnings, want)
	}
}

// Invalid attribute values are warnings, as in MJML's soft validation, on
// components and on the definitions of mj-attributes, after the illegal
// attributes of the same element, which make one warning. The messages are
// MJML 4.18's.
func TestAttributeValueWarnings(t *testing.T) {
	result, err := mjml.Compile("<mjml>\n<mj-head>\n<mj-attributes><mj-text padding=\"1em\" /><mj-all font-size=\"x\" /></mj-attributes>\n</mj-head>\n<mj-body>\n" +
		`<mj-section><mj-column><mj-text align="bogus" font-size="banana" color="nonsense" foo="1" bar="2">x</mj-text></mj-column></mj-section>` +
		"\n</mj-body>\n</mjml>")
	if err != nil {
		t.Fatal(err)
	}
	want := []mjml.Warning{
		{Kind: mjml.WarningInvalidAttributeValue, TagName: "mj-text", Line: 3, Message: "Attribute padding has invalid value: 1em for type Unit, only accepts (px, %) units and 1 to 4 value(s)"},
		{Kind: mjml.WarningInvalidAttribute, TagName: "mj-text", Line: 6, Message: "Attributes foo, bar are illegal"},
		{Kind: mjml.WarningInvalidAttributeValue, TagName: "mj-text", Line: 6, Message: "Attribute align has invalid value: bogus for type Enum, only accepts left, right, center, justify"},
		{Kind: mjml.WarningInvalidAttributeValue, TagName: "mj-text", Line: 6, Message: "Attribute font-size has invalid value: banana for type Unit, only accepts (px) units and 1 value(s)"},
		{Kind: mjml.WarningInvalidAttributeValue, TagName: "mj-text", Line: 6, Message: "Attribute color has invalid value: nonsense for type Color"},
	}
	if len(result.Warnings) != len(want) {
		t.Fatalf("got %d warnings, want %d: %+v", len(result.Warnings), len(want), result.Warnings)
	}
	for i := range want {
		if result.Warnings[i] != want[i] {
			t.Errorf("warning %d: got %+v, want %+v", i, result.Warnings[i], want[i])
		}
	}
}

// Characters XML does not allow, and bytes that are not UTF-8, are refused
// with their line, where MJML renders them.
func TestInvalidCharactersAreRefused(t *testing.T) {
	for c, want := range map[string]string{
		"\x0b": "illegal character code U+000B",
		"\x00": "illegal character code U+0000",
		"￾":    "illegal character code U+FFFE",
		"￿":    "illegal character code U+FFFF",
		"\xff": "invalid UTF-8",
	} {
		_, err := mjml.Render("<mjml><mj-body><mj-section><mj-column>\n<mj-text>a" + c + "b</mj-text></mj-column></mj-section></mj-body></mjml>")
		var e *mjml.Error
		if !errors.As(err, &e) || len(e.Details) != 1 || e.Details[0].Line != 2 || e.Details[0].Message != want {
			t.Errorf("%q: got %v, want %q at line 2", c, err, want)
		}
	}
}
