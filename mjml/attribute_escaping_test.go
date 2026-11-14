package mjml

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// wrapImage embeds a single mj-image snippet in a minimal valid MJML document.
func wrapImage(image string) string {
	return `<mjml><mj-body><mj-section><mj-column>` +
		image +
		`</mj-column></mj-section></mj-body></mjml>`
}

// renderImgAlt renders the MJML document, parses the resulting HTML the way a
// browser would, and returns the decoded (browser-visible) value of the alt
// attribute of the first <img> element together with the raw rendered HTML.
func renderImgAlt(t *testing.T, mjmlDoc string) (alt, rawHTML string) {
	t.Helper()
	rawHTML, err := Render(mjmlDoc)
	if err != nil {
		t.Fatalf("Render returned an error: %v", err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		t.Fatalf("failed to parse rendered HTML: %v", err)
	}
	img := doc.Find("img").First()
	if img.Length() == 0 {
		t.Fatalf("no <img> element found in rendered HTML:\n%s", rawHTML)
	}
	alt, _ = img.Attr("alt")
	return alt, rawHTML
}

// TestAttributeValueEscaping covers the four attribute-escaping cases from the
// bug report. In every case the author's intended alt text is `27" monitor` (or,
// for the angle-bracket case, `tag <b> here`), and the browser-visible alt value
// must read back exactly that text.
//
// Before the fix: case A and case C failed the XML parse outright, and case B
// rendered invalid markup (alt="27" monitor") whose embedded quote truncated the
// attribute. Case D, an escaped ampersand, displays its literal text as in MJML
// (it used to be decoded twice), and case E, a raw quote in a single-quoted
// value, is escaped rather than written raw as MJML does.
func TestAttributeValueEscaping(t *testing.T) {
	const wantQuote = `27" monitor`
	const wantAngle = `tag <b> here`

	cases := []struct {
		name  string
		image string
		want  string
	}{
		{
			name:  "A_named_quot_entity",
			image: `<mj-image src="https://e.test/a.png" alt="27&quot; monitor"/>`,
			want:  wantQuote,
		},
		{
			name:  "B_numeric_quote_entity",
			image: `<mj-image src="https://e.test/a.png" alt="27&#34; monitor"/>`,
			want:  wantQuote,
		},
		{
			name:  "C_named_angle_bracket_entities",
			image: `<mj-image src="https://e.test/a.png" alt="tag &lt;b&gt; here"/>`,
			want:  wantAngle,
		},
		{
			// An escaped ampersand is the text &: as in MJML, the alt
			// displays the entity's text, not a quote (safe-mjml used to
			// decode it twice).
			name:  "D_escaped_ampersand_is_literal",
			image: `<mj-image src="https://e.test/a.png" alt="27&amp;quot; monitor"/>`,
			want:  `27&quot; monitor`,
		},
		{
			name:  "E_single_quoted_value_with_a_quote",
			image: `<mj-image src="https://e.test/a.png" alt='27" monitor'/>`,
			want:  wantQuote,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, rawHTML := renderImgAlt(t, wrapImage(tc.image))
			if got != tc.want {
				t.Fatalf("browser-visible alt = %q, want %q\nrendered HTML:\n%s", got, tc.want, rawHTML)
			}
		})
	}
}

// TestAttributeValueEscapingSerialization asserts that the natural, XML-escaped
// input alt="27&quot; monitor" serializes with the quote kept encoded, so it can
// never terminate the attribute, rather than emitting a raw double quote.
func TestAttributeValueEscapingSerialization(t *testing.T) {
	_, rawHTML := renderImgAlt(t, wrapImage(`<mj-image src="https://e.test/a.png" alt="27&quot; monitor"/>`))

	if !strings.Contains(rawHTML, `alt="27&quot; monitor"`) {
		t.Fatalf("expected the quote to be emitted encoded as alt=\"27&quot; monitor\", got:\n%s", rawHTML)
	}
	if strings.Contains(rawHTML, `alt="27" monitor"`) {
		t.Fatalf("rendered a raw double quote inside the alt attribute (invalid markup):\n%s", rawHTML)
	}
}

// TestEndingTagContentKeepsEntities checks that the content of ending tags
// reaches the HTML as written, entities included, as MJML passes it: an
// escaped tag stays text (it used to be decoded into markup, or to fail the
// parse outside mj-text), and &amp; and &copy; stay as written.
func TestEndingTagContentKeepsEntities(t *testing.T) {
	content := `&copy; 2026 &amp; &lt;b&gt;bold&lt;/b&gt; &quot;q&quot;`
	for _, tag := range []string{"mj-text", "mj-button", "mj-table", "mj-navbar-link"} {
		inner := `<` + tag + `>` + content + `</` + tag + `>`
		switch tag {
		case "mj-navbar-link":
			inner = `<mj-navbar>` + inner + `</mj-navbar>`
		case "mj-table":
			inner = `<mj-table><tr><td>` + content + `</td></tr></mj-table>`
		}
		doc := `<mjml><mj-body><mj-section><mj-column>` + inner + `</mj-column></mj-section></mj-body></mjml>`
		out, err := Render(doc)
		if err != nil {
			t.Fatalf("%s: Render returned an error: %v", tag, err)
		}
		if !strings.Contains(out, content) {
			t.Errorf("%s: expected its content as written, %q, got:\n%s", tag, content, out)
		}
		if strings.Contains(out, "<b>bold</b>") {
			t.Errorf("%s: an escaped tag became markup:\n%s", tag, out)
		}
	}
}

// A raw double quote in a single-quoted value is escaped wherever the value
// is written in a double-quoted attribute, as everywhere else (Pierre's
// rule: values as written, only " escaped): the lang and dir of <html>, the
// class of a navbar link's Outlook cell, and the class of an accordion
// element's row. They were written raw, and the quote ended the attribute.
func TestRawQuoteIsEscapedInEverySink(t *testing.T) {
	doc := `<mjml lang='en"x' dir='ltr"x'><mj-body><mj-section><mj-column>` +
		`<mj-navbar><mj-navbar-link href="/a" css-class='nav"x'>A</mj-navbar-link></mj-navbar>` +
		`<mj-accordion><mj-accordion-element css-class='acc"x'><mj-accordion-title>T</mj-accordion-title><mj-accordion-text>X</mj-accordion-text></mj-accordion-element></mj-accordion>` +
		`</mj-column></mj-section></mj-body></mjml>`
	out, err := Render(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<html lang="en&quot;x" dir="ltr&quot;x"`,
		`class="nav&quot;x-outlook"`,
		`<tr class="acc&quot;x"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %s in:\n%s", want, out)
		}
	}
	for _, raw := range []string{`en"x`, `ltr"x`, `nav"x`, `acc"x`} {
		if strings.Contains(out, raw) {
			t.Errorf("%s written raw", raw)
		}
	}

	// The same classes set through an mj-class.
	out, err = Render(`<mjml><mj-head><mj-attributes><mj-class name="q" css-class='QQ"ZZ' /></mj-attributes></mj-head>` +
		`<mj-body><mj-section><mj-column>` +
		`<mj-navbar><mj-navbar-link href="/a" mj-class="q">A</mj-navbar-link></mj-navbar>` +
		`<mj-accordion><mj-accordion-element mj-class="q"><mj-accordion-title>T</mj-accordion-title><mj-accordion-text>X</mj-accordion-text></mj-accordion-element></mj-accordion>` +
		`</mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="QQ&quot;ZZ-outlook"`, `<tr class="QQ&quot;ZZ"`, `class="mj-link QQ&quot;ZZ"`} {
		if !strings.Contains(out, want) {
			t.Errorf("through an mj-class: want %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, `QQ"ZZ`) {
		t.Errorf(`QQ"ZZ written raw through an mj-class:\n%s`, out)
	}
}
