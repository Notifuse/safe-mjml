package mjml

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// scalingFamily builds a template of size n from a shape of input. Each
// family repeats one construction, valid or not: an author's mistake, a
// hostile template, or a large but ordinary email.
type scalingFamily struct {
	name  string
	n     int // the smaller size; the test also renders 4n
	build func(n int) string
}

func inColumn(body string) string {
	return "<mjml><mj-body><mj-section><mj-column>" + body + "</mj-column></mj-section></mj-body></mjml>"
}

// inColumnBody is the body of inColumn, for withHead.
func inColumnBody(body string) string {
	return "<mj-section><mj-column>" + body + "</mj-column></mj-section>"
}

func withHead(head, body string) string {
	return "<mjml><mj-head>" + head + "</mj-head><mj-body>" + body + "</mj-body></mjml>"
}

func repeat(n int, format string) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(strings.ReplaceAll(format, "%d", fmt.Sprint(i)))
	}
	return b.String()
}

var scalingFamilies = []scalingFamily{
	// Ordinary emails, large.
	{"sections", 200, func(n int) string {
		return "<mjml><mj-body>" + repeat(n, `<mj-section><mj-column><mj-text>Text %d</mj-text></mj-column></mj-section>`) + "</mj-body></mjml>"
	}},
	{"columns", 200, func(n int) string {
		return "<mjml><mj-body><mj-section>" + repeat(n, `<mj-column><mj-text>c%d</mj-text></mj-column>`) + "</mj-section></mj-body></mjml>"
	}},
	{"wrappers", 100, func(n int) string {
		return "<mjml><mj-body>" + repeat(n, `<mj-wrapper background-url="https://e.com/%d.png"><mj-section background-url="https://e.com/s.png"><mj-column><mj-button href="#">b%d</mj-button></mj-column></mj-section></mj-wrapper>`) + "</mj-body></mjml>"
	}},
	{"inline-html", 1000, func(n int) string {
		return inColumn("<mj-text>" + repeat(n, `<p class="p">a <b>b%d</b> <a href="https://e.com/?a=1&b=2">c</a></p>`) + "</mj-text>")
	}},
	{"navbar-links", 300, func(n int) string {
		return inColumn(`<mj-navbar hamburger="hamburger">` + repeat(n, `<mj-navbar-link href="/p%d">L%d</mj-navbar-link>`) + `</mj-navbar>`)
	}},
	{"social-elements", 300, func(n int) string {
		return inColumn(`<mj-social>` + repeat(n, `<mj-social-element name="facebook" href="https://e.com/%d">F</mj-social-element>`) + `</mj-social>`)
	}},
	{"accordion-elements", 200, func(n int) string {
		return inColumn(`<mj-accordion>` + repeat(n, `<mj-accordion-element><mj-accordion-title>T%d</mj-accordion-title><mj-accordion-text>X</mj-accordion-text></mj-accordion-element>`) + `</mj-accordion>`)
	}},
	// MJML 5's parser drops the mj-include tag and keeps what it holds:
	// indented siblings after one include, and many includes.
	{"include-then-siblings", 500, func(n int) string {
		return "<mjml><mj-body><mj-include path=\"x\"/>" + repeat(n, "\n                              <mj-raw>r%d</mj-raw>") + "</mj-body></mjml>"
	}},
	{"includes", 300, func(n int) string {
		return "<mjml><mj-body>" + repeat(n, `<mj-include path="p%d"> <mj-section><mj-column><mj-text>t</mj-text></mj-column></mj-section> </mj-include>`) + "</mj-body></mjml>"
	}},
	{"raw-conditionals", 500, func(n int) string {
		return "<mjml><mj-body>" + repeat(n, `<mj-raw><!--[if mso]><table><tr><td><![endif]--><p>r%d</p><!--[if mso]></td></tr></table><![endif]--></mj-raw>`) + "</mj-body></mjml>"
	}},
	// Head definitions.
	{"mj-class-definitions", 500, func(n int) string {
		return withHead("<mj-attributes>"+repeat(n, `<mj-class name="c%d" color="red" padding="1px" />`)+"</mj-attributes>",
			"<mj-section><mj-column>"+repeat(n, `<mj-text mj-class="c%d">t</mj-text>`)+"</mj-column></mj-section>")
	}},
	{"fonts", 500, func(n int) string {
		return withHead(repeat(n, `<mj-font name="F%d" href="https://e.com/%d.css" />`),
			"<mj-section><mj-column>"+repeat(n, `<mj-text font-family="F%d, Arial">t</mj-text>`)+"</mj-column></mj-section>")
	}},
	{"head-styles", 500, func(n int) string {
		return withHead(repeat(n, `<mj-style>.c%d { color: red; }</mj-style>`),
			"<mj-section><mj-column>"+repeat(n, `<mj-text css-class="c%d">t</mj-text>`)+"</mj-column></mj-section>")
	}},
	{"inline-rules-and-elements", 300, func(n int) string {
		// Many rules for one class, and many elements with that class.
		return withHead(`<mj-style inline="inline">`+strings.Repeat(`.a { color: red; }`, n)+`</mj-style>`,
			"<mj-section><mj-column>"+strings.Repeat(`<mj-text css-class="a">t</mj-text>`, n)+"</mj-column></mj-section>")
	}},
	{"inline-distinct-class-rules", 500, func(n int) string {
		return withHead(`<mj-style inline="inline">`+repeat(n, `.c%d { color: red; }`)+`</mj-style>`,
			"<mj-section><mj-column>"+repeat(n, `<mj-text css-class="c%d">t</mj-text>`)+"</mj-column></mj-section>")
	}},
	{"inline-descendant-chains", 100, func(n int) string {
		// n blocks of 40 nested divs: every element walks its 40 ancestors
		// once for each rule, so the cost is linear in n. Walking one tree
		// n deep is quadratic in n by nature (bounded by the nesting
		// limits); that chains of combinators no longer multiply that walk
		// is TestCombinatorChainsStopEarly's.
		return withHead(`<mj-style inline="inline">.absent div div div div div div { color: red; } div > div ~ div p { color: blue; }</mj-style>`,
			inColumnBody(`<mj-text>`+strings.Repeat(strings.Repeat(`<div>`, 40)+"t"+strings.Repeat(`</div>`, 40), n)+`</mj-text>`))
	}},
	{"inline-media-rules", 2000, func(n int) string {
		// mensch put a token back by copying every token left: a run of
		// at-rule blocks was quadratic in their number.
		return withHead(`<mj-style inline="inline">`+strings.Repeat(`@media x{p{color:red}}`, n)+`</mj-style>`,
			inColumnBody(`<mj-text><p>t</p></mj-text>`))
	}},
	{"inline-nested-media-rules", 500, func(n int) string {
		// mensch's lexer leaves a level open after each block, so the
		// stylesheet nests n deep and is refused past 100, in time linear
		// in its size (MJML's stack overflows on it).
		return withHead(`<mj-style inline="inline">`+strings.Repeat(`@media x{@media y{@supports (a:b){p{color:red}}}}`, n)+`</mj-style>`,
			inColumnBody(`<mj-text><p>t</p></mj-text>`))
	}},
	{"inline-style-attribute-declarations", 2000, func(n int) string {
		// One element's own style of n declarations, which a rule merges:
		// propMap scanned its slots for each, declarations² before the
		// index.
		return withHead(`<mj-style inline="inline">p { color: red; }</mj-style>`,
			inColumnBody(`<mj-text><p style="`+repeat(n, "p%d:1;")+`">t</p></mj-text>`))
	}},
	{"inline-long-selector-values", 20000, func(n int) string {
		// A long value written in a selector was lowered or scanned again
		// for each element tested.
		return withHead(`<mj-style inline="inline">[title=`+strings.Repeat("Y", n)+` i] { color: red; } p:not(.`+strings.Repeat("y", n)+`) { margin: 0; }</mj-style>`,
			inColumnBody(`<mj-text>`+strings.Repeat(`<p title class></p>`, n/10)+`</mj-text>`))
	}},
	{"inline-pseudo-classes", 1000, func(n int) string {
		return withHead(`<mj-style inline="inline">p:first-child { color: red; } p:last-of-type { color: blue; } p:not(.a):empty { margin: 0; }</mj-style>`,
			inColumnBody(`<mj-text>`+strings.Repeat(`<p>x</p><p></p>`, n)+`</mj-text>`))
	}},
	{"inline-stray-close-tags-deep", 5000, func(n int) string {
		// The HTML of an ending tag reaches juicer as deep as it is written:
		// each stray close tag walked every element open, and the depth
		// limit applied once the whole document was parsed.
		return withHead(`<mj-style inline="inline">p { color: red; }</mj-style>`,
			inColumnBody(`<mj-text>`+strings.Repeat(`<div>`, n)+strings.Repeat(`</x>`, n)+`</mj-text>`))
	}},
	{"inline-stray-close-tags-under-limit", 10000, func(n int) string {
		// Under the depth limit, so that the close tags are read: n/25
		// open elements, 1,600 at 4n, and n stray close tags, each of
		// which walked them all.
		return withHead(`<mj-style inline="inline">p { color: red; }</mj-style>`,
			inColumnBody(`<mj-text>`+strings.Repeat(`<div>`, n/25)+strings.Repeat(`</x>`, n)+`</mj-text>`))
	}},
	{"inline-quoted-close-tags", 1000, func(n int) string {
		// A quoted ">" in a close tag: each was read to the end of the run.
		return withHead(`<mj-style inline="inline">p { color: red; }</mj-style>`,
			inColumnBody(`<mj-text>`+strings.Repeat(`</b a='>'`, n)+`<p>x</p></mj-text>`))
	}},
	{"inline-html-attributes", 10000, func(n int) string {
		// One HTML tag with n attributes: each was looked up among those
		// before it.
		return withHead(`<mj-style inline="inline">p { color: red; }</mj-style>`,
			inColumnBody(`<mj-text><p `+repeat(n, `a%d `)+`>t</p></mj-text>`))
	}},
	{"inline-escaped-selectors", 20000, func(n int) string {
		// Escapes in a :contains() argument and in a selector name: each
		// byte counted the backslashes before it, and each escape copied
		// the name read so far.
		return withHead(`<mj-style inline="inline">p:contains(`+strings.Repeat(`\\\\`, n)+`) { color: red; } `+strings.Repeat(`\31 `, n)+`p { margin: 0; } .`+strings.Repeat(`\61`, n)+` { padding: 0; }</mj-style>`,
			inColumnBody(`<mj-text><p>x</p></mj-text>`))
	}},
	{"inline-template-blocks", 2000, func(n int) string {
		return withHead(`<mj-style inline="inline">p { color: red; }</mj-style>`,
			inColumnBody(`<mj-text>`+repeat(n, `<p title="{{ a%d }}">{% if x %}<%= y %>{{ z }}{% endif %}</p>`)+`</mj-text>`))
	}},
	// Long values and many attributes or classes.
	{"long-attribute", 20000, func(n int) string {
		return inColumn(`<mj-image src="https://e.com/a.png" alt="` + strings.Repeat("a&amp;b ", n) + `" />`)
	}},
	{"many-attributes", 2000, func(n int) string {
		return inColumn(`<mj-text ` + repeat(n, `data-a%d="x" `) + `>t</mj-text>`)
	}},
	{"many-classes", 2000, func(n int) string {
		return withHead(`<mj-style inline="inline">.a { color: red; }</mj-style>`,
			`<mj-section><mj-column><mj-text css-class="`+repeat(n, `a k%d `)+`">t</mj-text></mj-column></mj-section>`)
	}},
	{"text-quotes-and-angles", 20000, func(n int) string {
		return inColumn(`<mj-text>` + strings.Repeat(`"it's" a <3 b & c `, n) + `</mj-text>`)
	}},
	{"entities", 20000, func(n int) string {
		return inColumn(`<mj-text>` + strings.Repeat(`&eacute;&nbsp;&amp;&#60;&lt; `, n) + `</mj-text>`)
	}},
	{"huge-comment", 50000, func(n int) string {
		return "<mjml><mj-body><!--" + strings.Repeat("x <mj-text> ", n) + "--><mj-section><mj-column><mj-text>t</mj-text></mj-column></mj-section></mj-body></mjml>"
	}},
	{"many-comments", 2000, func(n int) string {
		return "<mjml><mj-body>" + repeat(n, `<!-- c%d --><mj-section><mj-column><mj-text>t</mj-text></mj-column></mj-section>`) + "</mj-body></mjml>"
	}},
	// Author mistakes and malformed input: the render may fail, but in
	// time linear in the input.
	{"unclosed-ending-tags", 2000, func(n int) string {
		return inColumn(strings.Repeat(`<mj-raw><mj-text>x</mj-text>`, n))
	}},
	{"ending-tags-closed-by-parent", 500, func(n int) string {
		return "<mjml><mj-body>" + strings.Repeat(`<mj-section><mj-column><mj-text>x</mj-column></mj-section>`, n) + "</mj-body></mjml>"
	}},
	{"stray-close-tags", 4000, func(n int) string {
		return "<mjml>" + strings.Repeat("<x>", n) + strings.Repeat("</y>", n) + strings.Repeat("</x>", n) + "</mjml>"
	}},
	{"deep-nesting", 200, func(n int) string {
		// Under the 1000-level limit at 4n.
		return "<mjml><mj-body>" + strings.Repeat("<x>", n) + "t" + strings.Repeat("</x>", n) + "</mj-body></mjml>"
	}},
	{"unclosed-comments", 2000, func(n int) string {
		return inColumn(strings.Repeat(`<mj-raw><!--[if mso]></mj-raw>`, n))
	}},
	{"unquoted-and-bare-attributes", 2000, func(n int) string {
		return inColumn(`<mj-raw>` + strings.Repeat(`<p class=a title=it's hidden>x</p>`, n) + `</mj-raw>`)
	}},
	{"unended-close-tags", 20000, func(n int) string {
		// No closing ">" anywhere, and no close of the document.
		return "<mjml><mj-body>" + strings.Repeat("</a ", n)
	}},
	{"unended-quoted-attributes", 20000, func(n int) string {
		return "<mjml><mj-body><mj-section><mj-column>" + strings.Repeat("<mj-text x='", n)
	}},
	{"close-tags-without-gt", 2000, func(n int) string {
		return inColumn(strings.Repeat("<mj-text>Hello\n</mj-column\n</mj-text>", n))
	}},
	{"stray-less-than", 20000, func(n int) string {
		return inColumn(`<mj-raw>` + strings.Repeat(`if a<b then 'x `, n) + `</mj-raw>`)
	}},
}

// mj-carousel is not a family: as in MJML, its CSS has a rule for every pair
// of images, so its output, and the time to write it, grow with the square
// of the number of images (an output size limit is planned after v1).

// knownSuperLinear lists the families that are known to cost more than
// linear time, with the reason. A listed family that becomes linear fails
// the test until its line is removed.
var knownSuperLinear = map[string]string{}

// TestRenderScalesLinearly renders each family at n and 4n and checks that
// 4 times the input takes less than 9 times as long (16 times for a
// quadratic cost), a ratio that does not depend on the machine or -race.
// The scaling tests written for one review finding each (fonts, nesting,
// unclosed ending tags) missed a quadratic parse of stray close tags; this
// test covers families of input shapes through the whole Render.
func TestRenderScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	measure := func(template string) time.Duration {
		best := time.Duration(0)
		for range 3 {
			start := time.Now()
			// A family may be refused (an author mistake), but never by a
			// panic, which Render returns as an ErrInternal.
			if _, err := Render(template); errors.Is(err, ErrInternal) {
				t.Fatalf("panic: %v", err)
			}
			if d := time.Since(start); best == 0 || d < best {
				best = d
			}
		}
		return best
	}
	seen := map[string]bool{}
	for _, f := range scalingFamilies {
		seen[f.name] = true
		t.Run(f.name, func(t *testing.T) {
			small, large := f.build(f.n), f.build(4*f.n)
			measure(small) // warm up
			base, got := measure(small), measure(large)
			ratio := float64(got) / float64(max(base, time.Microsecond))
			// Below a few milliseconds the timer and the GC dominate.
			superLinear := ratio > 9 && got > 20*time.Millisecond
			reason, known := knownSuperLinear[f.name]
			switch {
			case superLinear && known:
				t.Logf("known (%s): %d bytes in %v, %d bytes in %v (x%.1f)", reason, len(small), base, len(large), got, ratio)
			case superLinear:
				t.Errorf("%d bytes in %v, %d bytes in %v: x%.1f for x4 input", len(small), base, len(large), got, ratio)
			case known:
				t.Errorf("knownSuperLinear lists %s, which is now linear (x%.1f): remove it", f.name, ratio)
			default:
				t.Logf("%d bytes in %v, %d bytes in %v (x%.1f)", len(small), base, len(large), got, ratio)
			}
		})
	}
	for name := range knownSuperLinear {
		if !seen[name] {
			t.Errorf("knownSuperLinear lists %s, which is not a family", name)
		}
	}
}
