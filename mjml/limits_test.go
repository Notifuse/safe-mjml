package mjml_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Notifuse/safe-mjml/mjml"
)

// nested returns a template whose mj-body holds depth-2 nested <tag>
// elements, so that the deepest element is at nesting level depth.
func nested(tag string, depth int) string {
	n := depth - 2
	return "<mjml>\n<mj-body>" + strings.Repeat("<"+tag+">", n) + "t" + strings.Repeat("</"+tag+">", n) + "</mj-body></mjml>"
}

// A template nested too deeply is refused with a located error, instead of
// exhausting the stack, which ends the program (a fatal error, not a panic
// that recover could stop), and refusing it takes time linear in its depth:
// 4 times the depth in under 9 times the time, a ratio that does not depend
// on the speed of the machine or on the race detector. The depths are large
// (125,000 and 500,000 levels) and each time the fastest of 5, so that a
// loaded machine does not move the ratio.
func TestNestingTooDeepIsAnError(t *testing.T) {
	for name, run := range map[string]func(string) error{
		"Render": func(s string) error { _, err := mjml.Render(s); return err },
		"Parse":  func(s string) error { _, err := mjml.Parse(s); return err },
	} {
		mjmlErr := asError(t, run(nested("x", 1_000_000)))
		if len(mjmlErr.Details) != 1 || mjmlErr.Details[0].Line != 2 || mjmlErr.Details[0].TagName != "x" ||
			!strings.Contains(mjmlErr.Error(), "nested deeper than 1000 levels") {
			t.Errorf("%s: got %v", name, mjmlErr)
		}
		refuse := func(s string) {
			if err := run(s); err == nil || !strings.Contains(err.Error(), "nested deeper than 1000 levels") {
				t.Fatalf("%s: got %v, want the nesting error", name, err)
			}
		}
		if ratio := growthOfRuns(t, refuse, func(n int) string { return nested("x", n) }, 125_000, 5); ratio > 9 {
			t.Errorf("%s: 4 times the depth takes %.1f times as long to refuse", name, ratio)
		}
	}
}

func TestNestingAtTheLimit(t *testing.T) {
	// The deepest nesting allowed renders; one more level is refused.
	if _, err := mjml.Render(nested("x", 1000)); err != nil {
		t.Errorf("1000 levels: %v", err)
	}
	if _, err := mjml.Render(nested("x", 1001)); err == nil {
		t.Error("1001 levels: no error")
	}
	// Nested components, which the tree builder and the render walk too.
	if _, err := mjml.Render(nested("mj-wrapper", 1000)); err != nil {
		t.Errorf("1000 levels of mj-wrapper: %v", err)
	}
}

// Stray close tags are read in time linear in the document: each scanned the
// stack of open elements, so n stray closes under n open elements took time
// quadratic in n (5 s for 64000 of each) before the template was refused for
// its nesting.
func TestStrayClosesAreLinear(t *testing.T) {
	template := func(n int) string {
		return "<mjml>" + strings.Repeat("<x>", n) + strings.Repeat("</y>", n) + strings.Repeat("</x>", n) + "</mjml>"
	}
	render := func(source string) {
		if _, err := mjml.Render(source); err == nil || !strings.Contains(err.Error(), "nested deeper than") {
			t.Fatalf("got %v, want the nesting error", err)
		}
	}
	if ratio := growthOf(t, render, template, 8000); ratio > 9 {
		t.Errorf("4 times the input takes %.1f times as long", ratio)
	}
}

// Ending tags closed by their parent's close tag are read in time linear in
// the document: each is read once, up to that close.
func TestEndingTagsClosedByParentAreLinear(t *testing.T) {
	template := func(n int) string {
		return "<mjml><mj-body><mj-section>" + strings.Repeat("<mj-column><mj-text>x</mj-column>", n) + "</mj-section></mj-body></mjml>"
	}
	if ratio := growth(t, template, 1000); ratio > 9 {
		t.Errorf("4 times the input takes %.1f times as long", ratio)
	}
}

// Inline mj-style rules are applied in time linear in the document: many
// rules on one class, applied to many elements, or many selectors sharing a
// class, made every element read every rule before (rules × elements: 7.6 s
// for 10,000 rules and 5,000 elements).
func TestInlineRulesAreLinear(t *testing.T) {
	cases := map[string]func(n int) string{
		"many rules on one class": func(n int) string {
			return `<mjml><mj-head><mj-style inline="inline">` + strings.Repeat(".a { color: red; }\n", n) + `</mj-style></mj-head>` +
				`<mj-body><mj-section><mj-column>` + strings.Repeat(`<mj-text css-class="a">t</mj-text>`, n) + `</mj-column></mj-section></mj-body></mjml>`
		},
		"selectors sharing a class": func(n int) string {
			var rules, texts strings.Builder
			for i := range n {
				fmt.Fprintf(&rules, ".card.c%d { color: red; }\n", i)
				fmt.Fprintf(&texts, `<mj-text css-class="card c%d">t</mj-text>`, i)
			}
			return `<mjml><mj-head><mj-style inline="inline">` + rules.String() + `</mj-style></mj-head>` +
				`<mj-body><mj-section><mj-column>` + texts.String() + `</mj-column></mj-section></mj-body></mjml>`
		},
	}
	for name, template := range cases {
		if ratio := growth(t, template, 500); ratio > 9 {
			t.Errorf("%s: 4 times the input takes %.1f times as long", name, ratio)
		}
	}
}

// Tags that never end (no '>' after them) are read in time linear in the
// document: each made the pass that writes tag names in lower case scan to
// the end of the document again (80,000 of them, 312 KB, took 16 s). As in
// htmlparser2, nothing after a tag that never ends is a tag.
func TestUnendedTagsAreLinear(t *testing.T) {
	cases := map[string]func(n int) string{
		"close tags":         func(n int) string { return "<mjml><mj-body>" + strings.Repeat("</a ", n) },
		"open quoted values": func(n int) string { return "<mjml><mj-body>" + strings.Repeat("<mj-text x='", n) },
	}
	render := func(source string) { _, _ = mjml.Render(source) }
	for name, template := range cases {
		if ratio := growthOf(t, render, template, 5000); ratio > 9 {
			t.Errorf("%s: 4 times the input takes %.1f times as long", name, ratio)
		}
	}
}

// The fonts a document imports are found in time linear in its size: a
// repeated font-family: in an attribute or an inline rule, a long font name
// that almost matches, or many mj-font declarations took time quadratic in
// it before (seconds to minutes for a few hundred KB). Rendering 4 times the
// input must take about 4 times as long, not 16: the ratio does not depend
// on the speed of the machine or on the race detector.
func TestFontDiscoveryIsLinear(t *testing.T) {
	cases := map[string]func(n int) string{
		"attribute": func(n int) string {
			return `<mjml><mj-body><mj-section><mj-column><mj-text><span title="` + strings.Repeat("font-family:", n) + `">x</span></mj-text></mj-column></mj-section></mj-body></mjml>`
		},
		"inline rule": func(n int) string {
			return `<mjml><mj-head><mj-style inline="inline">.a{` + strings.Repeat("font-family:", n) + `}</mj-style></mj-head>` +
				`<mj-body><mj-section><mj-column><mj-text css-class="a">x</mj-text></mj-column></mj-section></mj-body></mjml>`
		},
		"long name": func(n int) string {
			return `<mjml><mj-head><mj-font name="` + strings.Repeat("a", n) + `b" href="https://x/a.css" /></mj-head>` +
				`<mj-body><mj-section><mj-column><mj-text font-family="` + strings.Repeat("a", 2*n) + `">x</mj-text></mj-column></mj-section></mj-body></mjml>`
		},
		"many declarations": func(n int) string {
			var fonts strings.Builder
			for i := range n {
				fmt.Fprintf(&fonts, `<mj-font name="F%d" href="https://x/%d.css" />`, i, i)
			}
			return "<mjml><mj-head>" + fonts.String() + "</mj-head><mj-body></mj-body></mjml>"
		},
	}
	for name, template := range cases {
		if ratio := growth(t, template, 4000); ratio > 9 {
			t.Errorf("%s: 4 times the input takes %.1f times as long", name, ratio)
		}
	}
}

// growth returns how many times longer rendering template(4n) takes than
// template(n), each the fastest of 3 renders, which must succeed.
func growth(t *testing.T, template func(int) string, n int) float64 {
	t.Helper()
	return growthOf(t, func(source string) {
		if _, err := mjml.Render(source); err != nil {
			t.Fatal(err)
		}
	}, template, n)
}

// growthOf is growth with its own render, for a template that must fail.
func growthOf(t *testing.T, render func(string), template func(int) string, n int) float64 {
	t.Helper()
	return growthOfRuns(t, render, template, n, 3)
}

// growthOfRuns is growthOf with each time the fastest of runs renders.
func growthOfRuns(t *testing.T, render func(string), template func(int) string, n, runs int) float64 {
	t.Helper()
	fastest := func(source string) time.Duration {
		best := time.Duration(1 << 62)
		for range runs {
			start := time.Now()
			render(source)
			best = min(best, time.Since(start))
		}
		return best
	}
	small, large := fastest(template(n)), fastest(template(4*n))
	return float64(large) / float64(max(small, time.Microsecond))
}

// The inline mj-style selectors cost a bounded work: 2,000 rules
// ".absent1 div" to ".absent2000 div", on 900 nested divs, walk the ancestors
// of every div for every rule (1.7 billion steps, 4.9 s without the limit),
// and the template is refused once juiceMaxMatchSteps are spent (1 s on an
// idle Apple M5). A template as large whose selectors cost little renders.
func TestInlineStyleMatchingIsBounded(t *testing.T) {
	if raceEnabled {
		t.Skip("250 million steps take 11 s under the race detector")
	}
	var rules strings.Builder
	for i := range 2000 {
		fmt.Fprintf(&rules, ".absent%d div { color: red; }\n", i)
	}
	nested := strings.Repeat("<div>", 900) + "t" + strings.Repeat("</div>", 900)
	template := func(css string) string {
		return `<mjml><mj-head><mj-style inline="inline">` + css + `</mj-style></mj-head>` +
			`<mj-body><mj-section><mj-column><mj-text>` + nested + `</mj-text></mj-column></mj-section></mj-body></mjml>`
	}
	_, err := mjml.Render(template(rules.String()))
	if err == nil || !strings.Contains(err.Error(), "matching the selectors to the document takes more than 250000000 steps") {
		t.Fatalf("got %v, want the limit of the inline mj-style", err)
	}
	var cheap strings.Builder
	for i := range 2000 {
		fmt.Fprintf(&cheap, ".present%d { color: red; }\n", i)
	}
	if _, err := mjml.Render(template(cheap.String())); err != nil {
		t.Errorf("rules on classes no element has: %v", err)
	}
}

// With an inline mj-style, juice reads the whole document: the HTML of an
// ending tag may nest up to twice parser.MaxNesting with the skeleton
// around it, and the inline stylesheet up to 100 blocks or parentheses,
// where parsing nested at-rules cost more than linear time (1,900 nested
// @media took 5 s). Deeper is refused; without an inline mj-style, the
// HTML of an ending tag is written as it is.
func TestInlineStyleDepthLimits(t *testing.T) {
	template := func(css string, spans int) string {
		return `<mjml><mj-head><mj-style inline="inline">` + css + `</mj-style></mj-head><mj-body><mj-section><mj-column><mj-text>` +
			strings.Repeat("<span>", spans) + "t" + strings.Repeat("</span>", spans) + `</mj-text></mj-column></mj-section></mj-body></mjml>`
	}
	nested := func(levels int) string {
		return strings.Repeat("@media screen{", levels) + "span{color:red}" + strings.Repeat("}", levels)
	}
	for _, tc := range []struct {
		name       string
		template   string
		refusedFor string // "" when it renders
	}{
		{"HTML 1,500 deep", template(".x{color:red}", 1500), ""},
		{"HTML 2,100 deep", template(".x{color:red}", 2100), "the document nests"},
		{"HTML 2,100 deep without an inline mj-style", strings.Replace(template("", 2100), ` inline="inline"`, "", 1), ""},
		{"a stylesheet 99 deep", template(nested(99), 1), ""},
		{"a stylesheet 101 deep", template(nested(101), 1), "a stylesheet nests"},
	} {
		_, err := mjml.Render(tc.template)
		switch {
		case tc.refusedFor == "" && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.refusedFor != "" && (err == nil || !strings.Contains(err.Error(), tc.refusedFor)):
			t.Errorf("%s: got %v, want an error with %q", tc.name, err, tc.refusedFor)
		}
	}
}

// The hostile inline mj-styles of the reviews of the juice port are refused
// before they cost seconds: each made the render take 2 to 192 s, or 2 GB
// of memory, under the step budget as it counted then (declarations, bytes
// read and lowered, :not() and stylesheet depth were free).
func TestHostileInlineStylesAreRefused(t *testing.T) {
	if raceEnabled {
		t.Skip("each spends 250 million steps, ten times slower under the race detector")
	}
	tpl := func(css, html string) string {
		return `<mjml><mj-head><mj-style inline="inline">` + css + `</mj-style></mj-head>` +
			`<mj-body><mj-section><mj-column><mj-text>` + html + `</mj-text></mj-column></mj-section></mj-body></mjml>`
	}
	rules := func(n int, format string) string {
		var b strings.Builder
		for i := range n {
			fmt.Fprintf(&b, format, i)
		}
		return b.String()
	}
	const steps, depth = "matching the selectors to the document takes more than", "a stylesheet nests"
	for _, tc := range []struct{ name, template, refusedFor string }{
		{"4,000 declarations on 1,000 elements", tpl(".a{"+rules(4000, "p%d:1;")+"}", strings.Repeat(`<p class="a"></p>`, 1000)), steps},
		{"4,500 rules on 4,500 elements", tpl(rules(4500, "p:not(.x%d){color:red}"), strings.Repeat("<p></p>", 4500)), steps},
		{":not() nested 1,000 deep", tpl(strings.Repeat(":not(", 1000)+".z"+strings.Repeat(")", 1000)+"{color:red}", "<p></p>"), depth},
		{"@media nested 1,900 deep", tpl(strings.Repeat("@media x{", 1900)+"p{color:red}"+strings.Repeat("}", 1900), "<p>x</p>"), depth},
		{"500 nested at-rule blocks, which mensch nests 500 deep", tpl(strings.Repeat("@media x{@media y{@supports (a:b){p{color:red}}}}", 500), "<p>x</p>"), depth},
		{"10,000 [title~=] rules on 2,000 long titles", tpl(rules(10000, "[title~=z%d]{color:red}"), strings.Repeat(`<p title="`+strings.Repeat("y", 1000)+`">x</p>`, 2000)), steps},
		{"13,000 :contains() on 2 MB of text", tpl(rules(13000, "p:contains(z%d){color:red}"), "<p>"+strings.Repeat("y", 2<<20)+"</p>"), steps},
		{"13,000 [title*= i] on a 700 KB title", tpl(rules(13000, "[title*=x%d i]{color:red}"), `<p title="`+strings.Repeat("Y", 700<<10)+`">x</p>`), steps},
		{"256 var() of a 10 KB property on 100 elements", tpl("html{--z:"+strings.Repeat("y", 10000)+"}p{a:"+strings.Repeat("var(--z) ", 256)+"}", strings.Repeat("<p></p>", 100)), "takes more than"},
		{"256 var() on 1,900 nested elements", tpl("span{a:"+strings.Repeat("var(--x)", 256)+"}", strings.Repeat("<span>", 1900)+"t"+strings.Repeat("</span>", 1900)), "takes more than"},
		{"a 100 KB value with 256 var() on 1,500 elements", tpl("p{a:"+strings.Repeat("var(--z)", 256)+strings.Repeat("y", 100000)+"}html{--z:q}", strings.Repeat("<p></p>", 1500)), "takes more than"},
		{"21,000 rules on an element of 40,000 classes", tpl(rules(21000, "x.c%d{color:red}"), `<p class="`+rules(40000, "c%d ")+`">x</p>`), steps},
		{"a 1 MB value on 300 elements", tpl("p{a:"+strings.Repeat("y", 1<<20)+"}", strings.Repeat("<p></p>", 300)), steps},
		// 30 MB of values: a byte of value costs 16 steps, so that what
		// the inliner holds stays near the 16 MB of the output.
		{"a 100 KB value on 300 elements", tpl("p{a:"+strings.Repeat("y", 100_000)+"}", strings.Repeat("<p></p>", 300)), steps},
		// Copies the budget saw only once they were made, gigabytes for
		// 100 KB (the reviews of 4ebbb74), and one it did not see at all.
		{"5,000 $` before 100 KB in a var()", tpl("p{--a:"+strings.Repeat("$`", 5000)+";color:"+strings.Repeat("y", 100000)+" var(--a)}", "<p>x</p>"), "takes more than"},
		{"5,000 $' after a var()", tpl("p{--a:"+strings.Repeat("$'", 5000)+";color:var(--a) "+strings.Repeat("y", 100000)+"}", "<p>x</p>"), "takes more than"},
		{"$&$& in a nested fallback", tpl("p{margin:var(--a,var(--b,$&$&))}", "<p>x</p>"), "takes more than"},
		{"a 100 KB template tag copied by a rule on 200 elements", tpl("p{a:"+strings.Repeat("juice_code_block_0_ ", 100)+"}", "{{"+strings.Repeat("y", 100000)+"}}"+strings.Repeat("<p></p>", 200)), "takes more than"},
		{"a 100 KB template tag copied 5,000 times as text", tpl("p{color:red}", "{{"+strings.Repeat("y", 100000)+"}}"+strings.Repeat("juice_code_block_0_ ", 5000)), "takes more than"},
		{"a var() of a 120 KB name on 1,900 nested elements", tpl("span{--"+strings.Repeat("A", 120000)+"x:1;a:var(--"+strings.Repeat("A", 120000)+"y)}", strings.Repeat("<span>", 1900)+"t"+strings.Repeat("</span>", 1900)), "takes more than"},
		{"a 1 MB selector", tpl("p"+strings.Repeat(" p", 520_000)+"{color:red}", "<p>x</p>"), "the selectors take more than"},
		{"5,300 selectors of 50 bytes", tpl(rules(5300, "p.c%d"+strings.Repeat(" p", 22)+"{color:red}"), "<p>x</p>"), "the selectors take more than"},
	} {
		_, err := mjml.Render(tc.template)
		if err == nil || !strings.Contains(err.Error(), tc.refusedFor) {
			t.Errorf("%s: got %v, want an error with %q", tc.name, err, tc.refusedFor)
		}
	}
}

// The input of the scaling family inline-descendant-chains-deep, which was
// quadratic by construction and was replaced on this branch by blocks of
// 40 nested divs (Pierre's decision): 800 nested divs with its two rules
// render, bounded by the nesting limits, without a linearity claim.
func TestDeepDescendantChainsRender(t *testing.T) {
	template := `<mjml><mj-head><mj-style inline="inline">.absent div div div div div div { color: red; } div > div ~ div p { color: blue; }</mj-style></mj-head>` +
		`<mj-body><mj-section><mj-column><mj-text>` + strings.Repeat("<div>", 800) + "t" + strings.Repeat("</div>", 800) + `</mj-text></mj-column></mj-section></mj-body></mjml>`
	if _, err := mjml.Render(template); err != nil {
		t.Fatal(err)
	}
}

// A value of mj-all or of an mj-class is written on every element it applies
// to: 150 KB of template wrote 240 MB with 1 GB of memory, as in MJML. The
// HTML of a render is bounded at 16 MB, and the render stops at the write
// that passes it.
func TestOutputIsBounded(t *testing.T) {
	value := strings.Repeat("z", 100_000)
	texts := strings.Repeat(`<mj-text mj-class="c">t</mj-text>`, 2400)
	for name, template := range map[string]string{
		"mj-all":   `<mjml><mj-head><mj-attributes><mj-all font-family="` + value + `"/></mj-attributes></mj-head><mj-body><mj-section><mj-column>` + texts + `</mj-column></mj-section></mj-body></mjml>`,
		"mj-class": `<mjml><mj-head><mj-attributes><mj-class name="c" padding="` + value + `"/></mj-attributes></mj-head><mj-body><mj-section><mj-column>` + texts + `</mj-column></mj-section></mj-body></mjml>`,
	} {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := mjml.Render(template)
		runtime.ReadMemStats(&after)
		if err == nil || !strings.Contains(err.Error(), "larger than 16 MB") {
			t.Errorf("%s: got %v, want the HTML refused for its size", name, err)
		}
		if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 200<<20 {
			t.Errorf("%s: refused after allocating %d MB", name, alloc>>20)
		}
	}
	// Under the limit, a large email renders.
	if _, err := mjml.Render(`<mjml><mj-body><mj-section><mj-column>` + strings.Repeat(`<mj-text>`+strings.Repeat("y", 1000)+`</mj-text>`, 5000) + `</mj-column></mj-section></mj-body></mjml>`); err != nil {
		t.Errorf("5 MB of text: %v", err)
	}
}

// WithLimits moves each limit of Limits: a field at zero keeps the default,
// a lower value refuses what the default renders, a higher one renders what
// the default refuses, and a negative one removes the limit. Every limit
// refused is reported by errors.Is(err, mjml.ErrLimitExceeded), the nesting
// limit too, and no other error is.
func TestWithLimits(t *testing.T) {
	style := func(css, body string) string {
		return `<mjml><mj-head><mj-style inline="inline">` + css + `</mj-style></mj-head><mj-body><mj-section><mj-column><mj-text>` + body + `</mj-text></mj-column></mj-section></mj-body></mjml>`
	}
	attributes := func(path, body string) string {
		return `<mjml><mj-head><mj-html-attributes><mj-selector path="` + path + `"><mj-html-attribute name="data-a">1</mj-html-attribute></mj-selector></mj-html-attributes></mj-head><mj-body><mj-raw>` + body + `</mj-raw></mj-body></mjml>`
	}
	long := "b" + strings.Repeat(" b", 140_000) // 280,001 bytes
	work := strings.Repeat("<p>", 700) + "x" + strings.Repeat("</p>", 700)
	text := `<mjml><mj-body><mj-section><mj-column><mj-text>` + strings.Repeat("y", 30_000) + `</mj-text></mj-column></mj-section></mj-body></mjml>`
	for _, tc := range []struct {
		name     string
		template string
		limits   mjml.Limits
		refused  bool
	}{
		{"a long inline selector, by default", style(long+"{color:red}", "x"), mjml.Limits{}, true},
		{"a long inline selector, under a higher limit", style(long+"{color:red}", "x"), mjml.Limits{SelectorBytes: 300_000}, false},
		{"a long inline selector, without a limit", style(long+"{color:red}", "x"), mjml.Limits{SelectorBytes: -1}, false},
		{"a short inline selector, under a lower limit", style("p.a{color:red}", "x"), mjml.Limits{SelectorBytes: 2}, true},
		{"a short inline selector, at the limit", style("p.a{color:red}", "x"), mjml.Limits{SelectorBytes: 3}, false},
		{"a long path, by default", attributes(long, "<b>x</b>"), mjml.Limits{}, true},
		{"a long path, under a higher limit", attributes(long, "<b>x</b>"), mjml.Limits{SelectorBytes: 300_000}, false},
		{"a short path, under a lower limit", attributes("td b", "<b>x</b>"), mjml.Limits{SelectorBytes: 3}, true},
		{"inline matching, under a lower work limit", style(".z p p{color:red}", work), mjml.Limits{WorkSteps: 30_000}, true},
		{"inline matching, by default", style(".z p p{color:red}", work), mjml.Limits{}, false},
		{"a path, under a lower work limit", attributes("p p", work), mjml.Limits{WorkSteps: 30_000}, true},
		{"30 KB of text, under a lower output limit", text, mjml.Limits{OutputSize: 20_000}, true},
		{"30 KB of text, by default", text, mjml.Limits{}, false},
		// The HTML grows by the style each <p> gets: about 11 KB before
		// inline mj-style, 31 KB after.
		{"inline mj-style growing the HTML past a lower output limit", style("p{color:red}", strings.Repeat("<p>x</p>", 1000)), mjml.Limits{OutputSize: 25_000}, true},
		{"30 KB of text, with a path, under a lower output limit", strings.Replace(text, "<mj-body>", `<mj-head><mj-html-attributes><mj-selector path="div"><mj-html-attribute name="data-a">1</mj-html-attribute></mj-selector></mj-html-attributes></mj-head><mj-body>`, 1), mjml.Limits{OutputSize: 20_000}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mjml.Render(tc.template, mjml.WithLimits(tc.limits))
			if tc.refused && !errors.Is(err, mjml.ErrLimitExceeded) {
				t.Errorf("got %v, want ErrLimitExceeded", err)
			}
			if !tc.refused && err != nil {
				t.Errorf("got %v, want the template rendered", err)
			}
		})
	}
	// The last WithLimits replaces the limits of those before it.
	if _, err := mjml.Render(style("p.a{color:red}", "x"), mjml.WithLimits(mjml.Limits{SelectorBytes: 2}), mjml.WithLimits(mjml.Limits{})); err != nil {
		t.Errorf("a later WithLimits{} keeps the defaults: %v", err)
	}
	if _, err := mjml.Render(nested("mj-wrapper", 1001)); !errors.Is(err, mjml.ErrLimitExceeded) {
		t.Errorf("nesting: got %v, want ErrLimitExceeded", err)
	}
	for _, template := range []string{"<mjml><mj-body>", "<mjml></mjml>", `<mjml><mj-body><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml><p>`} {
		if _, err := mjml.Render(template); err == nil || errors.Is(err, mjml.ErrLimitExceeded) {
			t.Errorf("%q: got %v, want an error that is not ErrLimitExceeded", template, err)
		}
	}
}

// The hostile mj-html-attributes of the review of the attribute pass are
// refused in bounded time and memory: they took 23 to 170 s and up to 8.6
// GB, as every attribute set, every byte of its value and every node walked
// were free. Each is refused by the step budget or the size of the output.
func TestHostileHTMLAttributesAreRefused(t *testing.T) {
	if raceEnabled {
		t.Skip("each spends up to 250 million steps, ten times slower under the race detector")
	}
	tpl := func(rules, raw string) string {
		return `<mjml><mj-head><mj-html-attributes>` + rules + `</mj-html-attributes></mj-head>` +
			`<mj-body><mj-raw>` + raw + `</mj-raw></mj-body></mjml>`
	}
	selector := func(path string, attrs ...string) string {
		var b strings.Builder
		b.WriteString(`<mj-selector path="` + path + `">`)
		for i := 0; i < len(attrs); i += 2 {
			b.WriteString(`<mj-html-attribute name="` + attrs[i] + `">` + attrs[i+1] + `</mj-html-attribute>`)
		}
		b.WriteString(`</mj-selector>`)
		return b.String()
	}
	many := func(n int) []string {
		out := make([]string, 0, 2*n)
		for i := range n {
			out = append(out, fmt.Sprintf("data-a%d", i), "1")
		}
		return out
	}
	selectors := func(n int) string {
		var b strings.Builder
		for i := range n {
			b.WriteString(selector(fmt.Sprintf("x%d", i), "data-x", "1"))
		}
		return b.String()
	}
	const steps, size = "takes more than 250000000 steps", "larger than 16 MB"
	for _, tc := range []struct{ name, template, refusedFor string }{
		{"a 100 KB value on 20,000 elements", tpl(selector("b", "data-v", strings.Repeat("v", 100_000)), strings.Repeat("<b/>", 20_000)), ""},
		{"a 100 KB value on 2,000 elements", tpl(selector("b", "data-v", strings.Repeat("v", 100_000)), strings.Repeat("<b/>", 2000)), ""},
		{"3,000 attributes on 3,000 elements", tpl(selector("b", many(3000)...), strings.Repeat("<b/>", 3000)), steps},
		{"6,000 attributes on 6,000 elements", tpl(selector("b", many(6000)...), strings.Repeat("<b/>", 6000)), steps},
		{"20,000 selectors over 200,000 comments", tpl(selectors(20_000), strings.Repeat("<!---->", 200_000)), steps},
		{"20,000 selectors over 20,000 elements", tpl(selectors(20_000), strings.Repeat("<b/>", 20_000)), steps},
		{"a 1 MB path", tpl(selector("b"+strings.Repeat(" b", 520_000), "data-x", "1"), "<b/>"), "the selectors take more than"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			_, err := mjml.Render(tc.template)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			switch {
			case err == nil:
				t.Fatal("rendered")
			case tc.refusedFor == "" && !strings.Contains(err.Error(), steps) && !strings.Contains(err.Error(), size):
				t.Errorf("got %v, want the step budget or the output size", err)
			case tc.refusedFor != "" && !strings.Contains(err.Error(), tc.refusedFor):
				t.Errorf("got %v, want an error with %q", err, tc.refusedFor)
			}
			// Coverage counters make each step about eight times slower.
			if limit := 10 * time.Second * time.Duration(coverSlowdown()); elapsed > limit {
				t.Errorf("refused after %v", elapsed)
			}
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 300<<20 {
				t.Errorf("refused after allocating %d MB", alloc>>20)
			}
		})
	}
}

// The attribute pass reads the body in XML, where every element left open
// nests what follows it in its parent: 2,537 <br> in an mj-raw nest 2,537
// deep, which MJML renders, as safe-mjml does (mj-html-attributes-deep
// compares the output with MJML's). Real depth is still bounded by the
// parser's nesting limit.
func TestHTMLAttributesDeepXML(t *testing.T) {
	template := `<mjml><mj-head><mj-html-attributes><mj-selector path="b"><mj-html-attribute name="data-d">1</mj-html-attribute></mj-selector></mj-html-attributes></mj-head>` +
		`<mj-body><mj-raw>` + strings.Repeat("<br>", 50_000) + `<b>x</b></mj-raw></mj-body></mjml>`
	html, err := mjml.Render(template)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `<b data-d="1">x</b>`) {
		t.Error("the attribute is not set after 50,000 <br>")
	}
}

// coverSlowdown is how many times slower a computation runs in this test
// binary: 10 with coverage counters (-cover), 1 without.
func coverSlowdown() int {
	if testing.CoverMode() != "" {
		return 10
	}
	return 1
}
