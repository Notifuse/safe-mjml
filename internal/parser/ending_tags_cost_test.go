package parser

import (
	"strings"
	"testing"
	"time"
)

// TestUnclosedEndingTagsCostLinear checks that ending tags left unclosed do
// not make the CDATA pass quadratic: each one used to scan to the end of the
// document again (a 560 KB template took 14 s). The first one now takes the
// rest of the document, as in MJML. The parse of such a document must cost
// about as much as that of a valid document of the same size.
func TestUnclosedEndingTagsCostLinear(t *testing.T) {
	const n = 20000
	wrap := func(body string) string {
		return "<mjml><mj-body><mj-section><mj-column>" + body + "</mj-column></mj-section></mj-body></mjml>"
	}
	unclosed := wrap(strings.Repeat("<mj-raw><mj-text>x</mj-text>", n))
	valid := wrap(strings.Repeat("<mj-raw>r</mj-raw><mj-text>x</mj-text>", n))
	elapsed := func(doc string) time.Duration {
		start := time.Now()
		_, _ = ParseMJML(doc)
		return time.Since(start)
	}
	elapsed(valid) // warm up
	base, got := elapsed(valid), elapsed(unclosed)
	if got > 10*base+200*time.Millisecond {
		t.Fatalf("parsing %d unclosed mj-raw took %v, a valid document of the same size %v", n, got, base)
	}
}

// TestEndingTagCloseVariants checks close tags MJML accepts: in another
// case, with something after the name, and after a quoted value holding the
// tag's name.
func TestEndingTagCloseVariants(t *testing.T) {
	for _, body := range []string{
		`<mj-text>a</MJ-TEXT><mj-text>b</mj-Text>`,
		`<mj-text>a</mj-text foo><mj-text>next</mj-text>`,
		`<mj-text><a title="<mj-text>">x</a> after</mj-text><mj-text>next</mj-text>`,
		`<mj-text><a title="</mj-text>">x</a> after</mj-text><mj-text>next</mj-text>`,
		`<mj-accordion><mj-accordion-element><mj-accordion-text>t</MJ-ACCORDION-TEXT></mj-accordion-element></mj-accordion>`,
		// A "<" that does not open a tag (no letter after it) is text.
		`<mj-text>I <3 you</mj-text><mj-text>next</mj-text>`,
		`<mj-text>under <5 euros</mj-text>`,
		`<mj-text>go <- back</mj-text><mj-text>next</mj-text>`,
		`<mj-text><p>We <3 you, don't we?</p></mj-text><mj-text>next</mj-text>`,
		`<mj-button href="#">Save <50 %</mj-button>`,
		`<mj-raw>{% if n <5 %}</mj-raw><mj-text>few</mj-text><mj-raw>{% endif %}</mj-raw>`,
	} {
		doc := "<mjml><mj-body><mj-section><mj-column>" + body + "</mj-column></mj-section></mj-body></mjml>"
		if _, err := ParseMJML(doc); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
}

// TestBareLessThanKeepsBlocksApart checks that a "<3" followed by an
// apostrophe does not swallow the close tag of its block: three mj-text stay
// three (they were merged into two, with no error).
func TestBareLessThanKeepsBlocksApart(t *testing.T) {
	doc := "<mjml><mj-body><mj-section><mj-column>" +
		"<mj-text>We <3 you, don't we?</mj-text><mj-text>It's <b>bold</b></mj-text><mj-text>third</mj-text>" +
		"</mj-column></mj-section></mj-body></mjml>"
	root, err := ParseMJML(doc)
	if err != nil {
		t.Fatal(err)
	}
	column := root.Children[0].Children[0].Children[0]
	if len(column.Children) != 3 {
		t.Fatalf("want 3 mj-text, got %d", len(column.Children))
	}
	if got := column.Children[0].Text; got != "We <3 you, don't we?" {
		t.Errorf("first mj-text = %q", got)
	}
}

// TestOutsideCDATAChangesOnlyOutside checks outsideCDATA, which copies the
// content only from the first outside part fn changes, against applying fn
// to each outside part.
func TestOutsideCDATAChangesOnlyOutside(t *testing.T) {
	upper := func(s string) string { return strings.ReplaceAll(s, "a", "A") }
	cases := map[string]string{
		"no change":           "<x>bcd</x>",
		"no CDATA":            "<x>abc</x>",
		"change first":        "<x>a<![CDATA[a]]>b</x>",
		"change last":         "<x>b<![CDATA[a]]>a</x>",
		"change in between":   "<x>b<![CDATA[a]]>a<![CDATA[a]]>c</x>",
		"only in CDATA":       "<x>b<![CDATA[aaa]]>c</x>",
		"comment with marker": "<x><!-- <![CDATA[ a -->a<![CDATA[a]]>a</x>",
		"unclosed CDATA":      "<x>a<![CDATA[a",
	}
	want := map[string]string{
		"no change":           "<x>bcd</x>",
		"no CDATA":            "<x>Abc</x>",
		"change first":        "<x>A<![CDATA[a]]>b</x>",
		"change last":         "<x>b<![CDATA[a]]>A</x>",
		"change in between":   "<x>b<![CDATA[a]]>A<![CDATA[a]]>c</x>",
		"only in CDATA":       "<x>b<![CDATA[aaa]]>c</x>",
		"comment with marker": "<x><!-- <![CDATA[ A -->A<![CDATA[a]]>A</x>",
		"unclosed CDATA":      "<x>A<![CDATA[A",
	}
	for name, in := range cases {
		if got := outsideCDATA(in, upper); got != want[name] {
			t.Errorf("%s: outsideCDATA(%q) = %q, want %q", name, in, got, want[name])
		}
	}
}
