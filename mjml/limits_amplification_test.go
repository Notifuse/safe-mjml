package mjml_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Notifuse/safe-mjml/mjml"
)

// amplificationCase is a template built to make one feature write or work
// as much as it can per byte of input: long values repeated on many
// elements, many rules on many elements, a selector evaluated again for
// every attribute. tags are the tags it exercises.
type amplificationCase struct {
	name     string
	tags     []string
	template string
}

// amplifyValue is the length of the values of the amplification templates:
// with about 20 attributes on 1,009 elements, a render would write 20 MB.
const amplifyValue = 997

// amplifyCount is the number of elements an amplification template repeats.
const amplifyCount = 1009

// amplifyPlacements gives, for each body component, amplifyCount instances
// of it in a place it may be, with what it needs to render.
var amplifyPlacements = map[string]func() string{
	"mj-body":    func() string { return times(`<mj-section><mj-column><mj-text>t</mj-text></mj-column></mj-section>`) },
	"mj-section": func() string { return times(`<mj-section><mj-column><mj-text>t</mj-text></mj-column></mj-section>`) },
	"mj-wrapper": func() string {
		return times(`<mj-wrapper><mj-section><mj-column><mj-text>t</mj-text></mj-column></mj-section></mj-wrapper>`)
	},
	"mj-hero": func() string { return times(`<mj-hero><mj-text>t</mj-text><mj-button>b</mj-button></mj-hero>`) },
	"mj-column": func() string {
		return times(`<mj-section><mj-column><mj-text>t</mj-text></mj-column><mj-column></mj-column></mj-section>`)
	},
	"mj-group": func() string {
		return times(`<mj-section><mj-group><mj-column><mj-text>t</mj-text></mj-column><mj-column></mj-column></mj-group></mj-section>`)
	},
	"mj-text":    func() string { return ampColumn(times(`<mj-text>t</mj-text>`)) },
	"mj-button":  func() string { return ampColumn(times(`<mj-button>b</mj-button>`)) },
	"mj-image":   func() string { return ampColumn(times(`<mj-image />`)) },
	"mj-divider": func() string { return ampColumn(times(`<mj-divider />`)) },
	"mj-spacer":  func() string { return ampColumn(times(`<mj-spacer />`)) },
	"mj-table":   func() string { return ampColumn(times(`<mj-table><tr><td>1</td></tr></mj-table>`)) },
	"mj-raw":     func() string { return ampColumn(times(`<mj-raw><p>r</p></mj-raw>`)) },
	"mj-social": func() string {
		return ampColumn(times(`<mj-social><mj-social-element name="facebook">f</mj-social-element></mj-social>`))
	},
	"mj-social-element": func() string {
		return ampColumn(`<mj-social>` + times(`<mj-social-element name="facebook">f</mj-social-element><mj-social-element>g</mj-social-element>`) + `</mj-social>`)
	},
	"mj-navbar": func() string {
		return ampColumn(times(`<mj-navbar hamburger="hamburger"><mj-navbar-link>l</mj-navbar-link></mj-navbar>`))
	},
	"mj-navbar-link": func() string {
		return ampColumn(`<mj-navbar hamburger="hamburger">` + times(`<mj-navbar-link>l</mj-navbar-link>`) + `</mj-navbar>`)
	},
	"mj-accordion": func() string {
		return ampColumn(times(`<mj-accordion><mj-accordion-element><mj-accordion-title>t</mj-accordion-title><mj-accordion-text>x</mj-accordion-text></mj-accordion-element></mj-accordion>`))
	},
	"mj-accordion-element": accordionElements,
	"mj-accordion-title":   accordionElements,
	"mj-accordion-text":    accordionElements,
	// The CSS of a carousel has a rule for every pair of its images, as in
	// MJML: many carousels of a few images each.
	"mj-carousel": func() string {
		return ampColumn(times(`<mj-carousel><mj-carousel-image /><mj-carousel-image /></mj-carousel>`))
	},
	"mj-carousel-image": func() string {
		return ampColumn(strings.Repeat(`<mj-carousel>`+strings.Repeat(`<mj-carousel-image />`, 7)+`</mj-carousel>`, amplifyCount/7))
	},
}

func times(s string) string { return strings.Repeat(s, amplifyCount) }

func ampColumn(body string) string {
	return "<mj-section><mj-column>" + body + "</mj-column></mj-section>"
}

func ampHead(head, body string) string {
	return "<mjml><mj-head>" + head + "</mj-head><mj-body>" + body + "</mj-body></mjml>"
}

// ampRepeat repeats format amplifyCount times, %d replaced by the index.
func ampRepeat(format string) string {
	var b strings.Builder
	for i := range amplifyCount {
		b.WriteString(strings.ReplaceAll(format, "%d", strconv.Itoa(i)))
	}
	return b.String()
}

func accordionElements() string {
	return ampColumn(`<mj-accordion>` + times(`<mj-accordion-element><mj-accordion-title>t</mj-accordion-title><mj-accordion-text>x</mj-accordion-text></mj-accordion-element>`) + `</mj-accordion>`)
}

// amplificationCases returns a case per body component, which gives every
// attribute it accepts a long value through mj-attributes and repeats it,
// with the head features' own cases.
func amplificationCases(t *testing.T) []amplificationCase {
	data, err := os.ReadFile("../internal/components/allowed-css-attributes.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed map[string]map[string]string
	if err := json.Unmarshal(data, &allowed); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("z", amplifyValue)
	// A value of a head feature is written on every element it reaches:
	// longer, so that 1,009 elements would pass the 16 MB of the output.
	longer := strings.Repeat(long, 17)
	var cases []amplificationCase
	for _, tag := range slices.Sorted(maps.Keys(amplifyPlacements)) {
		var attrs strings.Builder
		for _, name := range slices.Sorted(maps.Keys(allowed[tag])) {
			fmt.Fprintf(&attrs, ` %s="%s"`, name, long)
		}
		cases = append(cases, amplificationCase{
			name:     tag + " with every attribute long",
			tags:     []string{tag, "mj-attributes"},
			template: ampHead(`<mj-attributes><`+tag+attrs.String()+` /></mj-attributes>`, amplifyPlacements[tag]()),
		})
	}
	texts := ampColumn(times(`<mj-text mj-class="c" css-class="k">t</mj-text>`))
	var classes, selectors, attributes strings.Builder
	for i := range 101 {
		fmt.Fprintf(&classes, `<mj-class name="c%d" padding="%s" />`, i, long)
		fmt.Fprintf(&selectors, `<mj-selector path="td:not(:first) div, :has(div):lt(%d)"><mj-html-attribute name="data-%d">%s</mj-html-attribute></mj-selector>`, i, i, long)
	}
	for i := range 3001 {
		fmt.Fprintf(&attributes, `<mj-html-attribute name="data-a%d">1</mj-html-attribute>`, i)
	}
	classList := strings.TrimSpace(strings.Repeat("c1 c2 c3 c5 c8 c13 c21 c34 c55 c89 ", 10))
	return append(cases,
		amplificationCase{"a long mj-title and mj-preview", []string{"mj-head", "mj-title", "mj-preview"},
			ampHead(`<mj-title>`+strings.Repeat(long, 1009)+`</mj-title><mj-preview>`+strings.Repeat(long, 1009)+`</mj-preview>`, texts)},
		amplificationCase{"many fonts, each used", []string{"mj-font"},
			ampHead(ampRepeat(`<mj-font name="F%d" href="https://e.com/`+long+`%d.css" />`),
				ampColumn(ampRepeat(`<mj-text font-family="F%d, Arial">t</mj-text>`)))},
		amplificationCase{"a long breakpoint and many column widths", []string{"mj-breakpoint"},
			ampHead(`<mj-breakpoint width="`+longer+`px" />`, ampRepeat(`<mj-section><mj-column width="%dpx"><mj-text>t</mj-text></mj-column></mj-section>`))},
		amplificationCase{"many mj-style blocks", []string{"mj-style"},
			ampHead(times(`<mj-style>.k { a: `+long+`; }</mj-style>`), texts)},
		amplificationCase{"an inline mj-style with a long declaration", []string{"mj-style"},
			ampHead(`<mj-style inline="inline">div, td, table, tr { a: `+longer+`; }</mj-style>`, texts)},
		amplificationCase{"mj-all with a long value", []string{"mj-all"},
			ampHead(`<mj-attributes><mj-all font-family="`+longer+`" padding="`+longer+`" /></mj-attributes>`, texts)},
		amplificationCase{"an mj-class list of long classes", []string{"mj-class"},
			ampHead(`<mj-attributes>`+classes.String()+`</mj-attributes>`,
				ampColumn(times(`<mj-text mj-class="`+classList+`">t</mj-text>`)))},
		amplificationCase{"mj-html-attributes: a long value on every element", []string{"mj-html-attributes", "mj-selector", "mj-html-attribute"},
			ampHead(`<mj-html-attributes><mj-selector path="*"><mj-html-attribute name="data-v">`+longer+`</mj-html-attribute></mj-selector></mj-html-attributes>`, texts)},
		amplificationCase{"mj-html-attributes: many attributes on every element", []string{"mj-html-attributes"},
			ampHead(`<mj-html-attributes><mj-selector path="*">`+attributes.String()+`</mj-selector></mj-html-attributes>`, texts)},
		amplificationCase{"mj-html-attributes: costly selectors evaluated per attribute", []string{"mj-html-attributes"},
			ampHead(`<mj-html-attributes>`+selectors.String()+`</mj-html-attributes>`, texts)},
		// MJML 5's parser drops the mj-include tag and keeps what it holds.
		amplificationCase{"mj-include holding text", []string{"mj-include"},
			"<mjml><mj-body>" + ampRepeat(`<mj-include path="p%d"> `+long+` <mj-raw>r</mj-raw></mj-include>`) + strings.Repeat(`<mj-include path="q"><mj-include path="r">`+long+`<mj-raw>s</mj-raw>`, 211) + strings.Repeat("</mj-include>", 422) + "</mj-body></mjml>"},
	)
}

// amplificationTagsBeyondComponents are the tags with a meaning of their own
// that are not components: each needs a case too.
var amplificationTagsBeyondComponents = []string{"mj-class", "mj-selector", "mj-html-attribute", "mj-include"}

// The amplification guard: every template built to make a feature write or
// work as much as it can per byte of input renders, or is refused by a limit,
// in bounded time and memory. A new component or head tag fails the test
// until it has a case, so that the guard does not depend on remembering
// which paths can amplify. The bounds are well above what the cases take
// (each under half a second and 200 MB here), and well below what an
// unbounded feature takes: 150 KB of mj-all allocated 1 GB before the output
// limit, and mj-html-attributes had no bound of its own.
func TestAmplificationIsBounded(t *testing.T) {
	if raceEnabled {
		t.Skip("the time bound assumes no race detector")
	}
	const maxAlloc = 400 << 20
	maxDuration := 8 * time.Second * time.Duration(coverSlowdown())
	cases := amplificationCases(t)
	covered := map[string]bool{}
	for _, c := range cases {
		for _, tag := range c.tags {
			covered[tag] = true
		}
		t.Run(c.name, func(t *testing.T) {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			html, err := mjml.Render(c.template)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			alloc := after.TotalAlloc - before.TotalAlloc
			t.Logf("%d KB in, %d KB out (%v), %v, %d MB allocated", len(c.template)>>10, len(html)>>10, err, elapsed.Round(time.Millisecond), alloc>>20)
			if errors.Is(err, mjml.ErrInternal) {
				t.Fatalf("panic: %v", err)
			}
			if elapsed > maxDuration {
				t.Errorf("took %v, more than %v", elapsed, maxDuration)
			}
			if alloc > maxAlloc {
				t.Errorf("allocated %d MB, more than %d MB", alloc>>20, maxAlloc>>20)
			}
		})
	}
	for _, tag := range append(mjml.ComponentTags(), amplificationTagsBeyondComponents...) {
		if !covered[tag] {
			t.Errorf("%s has no amplification case: add one to amplificationCases", tag)
		}
	}
}
