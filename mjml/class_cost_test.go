package mjml

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// classCostDocuments returns the two hostile documents of the review of
// v0.18.0 (or, without heavy, the same without their class work), for
// TestClassTagDefaultsCost.
func classCostDocuments(heavy bool) map[string]string {
	docs := map[string]string{}

	// A long class list naming no defined class, on a section with many
	// texts: the list must be read once, not once per descendant.
	classes := ""
	if heavy {
		classes = ` mj-class="` + strings.Repeat("missing ", 20000) + `"`
	}
	var texts strings.Builder
	for i := 0; i < 2000; i++ {
		texts.WriteString("<mj-text>t</mj-text>")
	}
	docs["long class list"] = `<mjml><mj-body><mj-section` + classes + `><mj-column>` + texts.String() + `</mj-column></mj-section></mj-body></mjml>`

	// A class tag with many attributes, inherited by an element with as many
	// explicit attributes: each default must not scan the element's
	// attributes.
	var defaults, explicit strings.Builder
	for i := 0; i < 32000; i++ { // the review's generator: about 1 MB
		n := strconv.Itoa(i)
		defaults.WriteString(` data-a` + n + `="1"`)
		explicit.WriteString(` data-b` + n + `="2"`)
	}
	head := ""
	sectionClass := ""
	if heavy {
		head = `<mj-head><mj-attributes><mj-class name="c"><mj-text` + defaults.String() + ` /></mj-class></mj-attributes></mj-head>`
		sectionClass = ` mj-class="c"`
	}
	docs["many defaults and attributes"] = `<mjml>` + head + `<mj-body><mj-section` + sectionClass + `><mj-column><mj-text` + explicit.String() + `>t</mj-text></mj-column></mj-section></mj-body></mjml>`
	return docs
}

// TestClassTagDefaultsCost bounds the cost of the tags inside an mj-class
// (classesDefault) against the same document without classes: a long class
// list must not multiply the bytes allocated (it was read once per
// descendant), and many inherited defaults over many explicit attributes
// must not multiply the time (each default scanned the element's
// attributes). Both were quadratic in the template, which Notifuse users
// write. The bounds are loose; the quadratic code exceeded them tenfold.
func TestClassTagDefaultsCost(t *testing.T) {
	light, heavy := classCostDocuments(false), classCostDocuments(true)
	render := func(t *testing.T, doc string) {
		if _, err := Render(doc, withWarningHandler(func(Warning) {})); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("long class list", func(t *testing.T) {
		bytes := func(doc string) uint64 {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			render(t, doc)
			runtime.ReadMemStats(&after)
			return after.TotalAlloc - before.TotalAlloc
		}
		name := "long class list"
		base, cost := bytes(light[name]), bytes(heavy[name])
		t.Logf("%d bytes with the class list, %d without", cost, base)
		if cost > 3*base+(4<<20) {
			t.Errorf("%d bytes allocated with the class list against %d without: the list is not read once", cost, base)
		}
	})

	t.Run("many defaults and attributes", func(t *testing.T) {
		elapsed := func(doc string) time.Duration {
			best := time.Duration(1 << 62)
			for i := 0; i < 3; i++ {
				start := time.Now()
				render(t, doc)
				best = min(best, time.Since(start))
			}
			return best
		}
		name := "many defaults and attributes"
		base, cost := elapsed(light[name]), elapsed(heavy[name])
		t.Logf("%v with the class defaults, %v without", cost, base)
		if cost > 5*base+100*time.Millisecond {
			t.Errorf("%v with the class defaults against %v without: each default scans the element's attributes", cost, base)
		}
	})
}

// repeatedClassDocuments returns documents that repeat one class n times in
// an mj-class (or name it once, without repeated), for
// TestRepeatedClassCost: its css-class, which each occurrence adds to the
// element's class, and its own attributes and inner mj-text tag, which only
// the last occurrence decides.
func repeatedClassDocuments(repeated bool) map[string]string {
	list := func(n int) string {
		if !repeated {
			return "c"
		}
		return strings.TrimSuffix(strings.Repeat("c ", n), " ")
	}
	docs := map[string]string{}
	docs["css-class"] = `<mjml><mj-head><mj-attributes><mj-class name="c" css-class="x" /></mj-attributes></mj-head>` +
		`<mj-body><mj-section><mj-column><mj-text mj-class="` + list(32000) + `">t</mj-text></mj-column></mj-section></mj-body></mjml>`

	var attrs strings.Builder
	for i := 0; i < 8000; i++ {
		attrs.WriteString(` data-a` + strconv.Itoa(i) + `="1"`)
	}
	docs["attributes"] = `<mjml><mj-head><mj-attributes><mj-class name="c"` + attrs.String() + ` /></mj-attributes></mj-head>` +
		`<mj-body><mj-section><mj-column><mj-text mj-class="` + list(8000) + `">t</mj-text></mj-column></mj-section></mj-body></mjml>`
	docs["class tags"] = `<mjml><mj-head><mj-attributes><mj-class name="c"><mj-text` + attrs.String() + ` /></mj-class></mj-attributes></mj-head>` +
		`<mj-body><mj-section mj-class="` + list(8000) + `"><mj-column><mj-text>t</mj-text></mj-column></mj-section></mj-body></mjml>`
	return docs
}

// TestRepeatedClassCost bounds the cost of a class repeated in an mj-class
// against the same class named once: the css-class of each occurrence must be
// joined once (adding it to a growing string was quadratic), and the
// attributes of the class and of its inner tags must be merged once per
// distinct class, not once per occurrence (occurrences × attributes). The
// bounds are loose; the quadratic code exceeded them tenfold.
func TestRepeatedClassCost(t *testing.T) {
	once, repeated := repeatedClassDocuments(false), repeatedClassDocuments(true)
	for _, name := range []string{"css-class", "attributes", "class tags"} {
		t.Run(name, func(t *testing.T) {
			measure := func(doc string) (uint64, time.Duration) {
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				start := time.Now()
				if _, err := Render(doc, withWarningHandler(func(Warning) {})); err != nil {
					t.Fatal(err)
				}
				elapsed := time.Since(start)
				runtime.ReadMemStats(&after)
				return after.TotalAlloc - before.TotalAlloc, elapsed
			}
			baseBytes, baseTime := measure(once[name])
			bytes, elapsed := measure(repeated[name])
			t.Logf("repeated: %d bytes, %v; once: %d bytes, %v", bytes, elapsed, baseBytes, baseTime)
			if bytes > 4*baseBytes+(8<<20) {
				t.Errorf("%d bytes allocated for the repeated class against %d for one", bytes, baseBytes)
			}
			if elapsed > 5*baseTime+150*time.Millisecond {
				t.Errorf("%v for the repeated class against %v for one", elapsed, baseTime)
			}
		})
	}
}
