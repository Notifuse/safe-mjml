package mjml

import (
	"fmt"
	"strings"
	"testing"
)

// TestHTMLAttributesInvalidSelector checks that an mj-selector whose path
// cheerio cannot read sets nothing and gives a warning on its line, where
// MJML fails, and that the other selectors still apply.
func TestHTMLAttributesInvalidSelector(t *testing.T) {
	result, err := Compile(`<mjml>
  <mj-head>
    <mj-html-attributes>
      <mj-selector path="td["><mj-html-attribute name="data-bad">1</mj-html-attribute></mj-selector>
      <mj-selector path=".x td"><mj-html-attribute name="data-good">2</mj-html-attribute></mj-selector>
    </mj-html-attributes>
  </mj-head>
  <mj-body><mj-section css-class="x"><mj-column><mj-text>t</mj-text></mj-column></mj-section></mj-body>
</mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "data-bad") || !strings.Contains(result.HTML, `data-good="2"`) {
		t.Errorf("the invalid selector set an attribute, or the valid one none:\n%s", result.HTML)
	}
	var found bool
	for _, w := range result.Warnings {
		if w.Line == 4 && strings.Contains(w.Message, `mj-selector path "td[" is not a selector safe-mjml reads: it sets no attribute`) {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning on line 4 for the invalid selector: %v", result.Warnings)
	}
}

// TestHTMLAttributesSelectorsMJMLRejects checks the paths on which MJML
// fails ("Pseudo-elements are not supported by css-select", "Empty
// sub-selector", "Unknown pseudo-class"): each sets nothing, with a warning,
// even where juice would read it.
func TestHTMLAttributesSelectorsMJMLRejects(t *testing.T) {
	// Selectors nested 10,000 deep, which overflow MJML's stack, also where a
	// quote or a parenthesis would hide the nesting from a count that does
	// not read the selector as the parser does.
	deep := func(before string) string {
		return before + strings.Repeat(":not(", 10_000) + ":first" + strings.Repeat(")", 10_000)
	}
	for _, path := range []string{
		"a::before", "td:before", "a::first-line", "td, , a", "td,", ",td", ":focus",
		deep("zz"), deep("zz:contains(')"), deep("zz:icontains(it's)"), deep("zz[a=" + strings.Repeat(")", 10_000) + "]"),
	} {
		name := path
		if len(name) > 40 {
			name = name[:40]
		}
		t.Run(name, func(t *testing.T) {
			result, err := Compile(`<mjml>
  <mj-head>
    <mj-html-attributes>
      <mj-selector path="` + path + `"><mj-html-attribute name="data-bad">131</mj-html-attribute></mj-selector>
    </mj-html-attributes>
  </mj-head>
  <mj-body><mj-section><mj-column><mj-text>t</mj-text><mj-button href="https://example.com">b</mj-button></mj-column></mj-section></mj-body>
</mjml>`)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(result.HTML, "data-bad") {
				t.Errorf("the path set an attribute:\n%s", result.HTML)
			}
			want := fmt.Sprintf("mj-selector path %q is not a selector safe-mjml reads: it sets no attribute", path)
			if len(result.Warnings) != 1 || result.Warnings[0].Line != 4 || result.Warnings[0].Message != want {
				t.Errorf("warnings: %v, want one on line 4: %s", result.Warnings, want)
			}
		})
	}
}

// TestHTMLAttributesOutlookConditionals checks that the pass reads the body
// with its Outlook conditionals unmerged, as mjml-core does, also without an
// inline mj-style: a stray <![endif]--> in an mj-raw leaves the section's
// Outlook table inside a comment, out of the pass's reach, as in MJML
// 5.4.1, which sets the attribute on the two other cells only and adds no
// end tag.
func TestHTMLAttributesOutlookConditionals(t *testing.T) {
	result, err := Compile(`<mjml>
  <mj-head>
    <mj-html-attributes>
      <mj-selector path="td"><mj-html-attribute name="data-b">137</mj-html-attribute></mj-selector>
    </mj-html-attributes>
  </mj-head>
  <mj-body><mj-raw><![endif]--></mj-raw><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body>
</mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<td style="line-height:0px;font-size:0px;mso-line-height-rule:exactly;"><![endif]-->`,
		`<td style="direction:ltr;font-size:0px;padding:20px 0;text-align:center;" data-b="137">`,
		`<td align="left" style="font-size:0px;padding:10px 25px;word-break:break-word;" data-b="137">`,
		`<!--[if mso | IE]></td></tr></table><![endif]--></div></body>`,
	} {
		if !strings.Contains(result.HTML, want) {
			t.Errorf("missing %s in:\n%s", want, result.HTML)
		}
	}
	if n := strings.Count(result.HTML, "data-b="); n != 2 {
		t.Errorf("%d cells have data-b, want 2:\n%s", n, result.HTML)
	}
}

// TestHTMLAttributeTextContent checks the value of an mj-html-attribute as
// MJML's parser reads it (values verified with MJML 5.4.1): text chunks split
// at each "<", trimmed and joined, child elements, comments and processing
// instructions left out, a CDATA section's content kept, and JavaScript's
// trim, which removes U+FEFF and keeps U+0085.
func TestHTMLAttributeTextContent(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{"a < b", "a< b"},
		{"1 <2", "1<2"},
		{"a <!-- c --> b", "ab"},
		{"a <b>c</b> d", "ad"},
		{"a <b>c<i>e</i>f</b> d", "ad"},
		{"x<i>y", "x"},
		{"x<br>y", "xy"},
		{"x<br/>y", "xy"},
		{"x<img src='>'>y", "xy"},
		{`<![CDATA[a "q" <b>]]>`, `a "q" <b>`},
		{"a<?pi?>b", "ab"},
		{"a<!DOCTYPE x>b", "ab"},
		{"\u0085x\u0085", "\u0085x\u0085"},
		{"\ufeffy\ufeff", "y"},
		{" plain value ", "plain value"},
		{"a\n  b", "a\n  b"},
		{"&quot;q&quot;", "&quot;q&quot;"},
		{"", ""},
	} {
		if got := textContent(c.raw); got != c.want {
			t.Errorf("textContent(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
