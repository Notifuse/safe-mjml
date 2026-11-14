package mjml

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// referenceRandomID matches the 16-hex-digit IDs MJML generates for mj-carousel
// and the mj-navbar checkbox, where they appear: after "mj-carousel-" or
// "mj-carousel-radio-", and as an id or for value. It is the rule of RANDOM_ID
// in tools/mjml-reference/regenerate.mjs, without its "#" alternative, which
// no output uses. The whole hex run is captured so that a longer run can be
// told apart.
var referenceRandomID = regexp.MustCompile(`(?:mj-carousel-(?:radio-)?|id="|for=")([0-9a-f]{16,})`)

// referenceIDGenerator returns an ID generator for WithIDGenerator that hands
// out the IDs of reference, distinct and in order of first appearance, which
// is the order in which the render asks for them. Once they run out, it
// returns 0000000000000001, 0000000000000002…, which differ from the
// reference and make the comparison fail.
func referenceIDGenerator(reference string) func() string {
	var ids []string
	seen := map[string]bool{}
	for _, m := range referenceRandomID.FindAllStringSubmatch(reference, -1) {
		if id := m[1]; len(id) == 16 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return sequentialIDGenerator(ids...)
}

// sequentialIDGenerator returns an ID generator that hands out ids, then
// 0000000000000001, 0000000000000002… It is not safe for concurrent use: give
// each render its own.
func sequentialIDGenerator(ids ...string) func() string {
	next, extra := 0, 0
	return func() string {
		if next < len(ids) {
			next++
			return ids[next-1]
		}
		extra++
		return fmt.Sprintf("%016x", extra)
	}
}

const carouselAndNavbar = `<mjml><mj-body><mj-section><mj-column>
<mj-carousel><mj-carousel-image src="https://example.com/a.jpg" /><mj-carousel-image src="https://example.com/b.jpg" /></mj-carousel>
<mj-navbar hamburger="hamburger"><mj-navbar-link href="/a">A</mj-navbar-link></mj-navbar>
</mj-column></mj-section></mj-body></mjml>`

func TestWithIDGenerator(t *testing.T) {
	first, err := Render(carouselAndNavbar, WithIDGenerator(sequentialIDGenerator("aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb")))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(carouselAndNavbar, WithIDGenerator(sequentialIDGenerator("aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb")))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("two renders with the same generator differ")
	}
	// The carousel ID comes first: its CSS, in the head, appears before the
	// navbar checkbox in the body.
	if !strings.Contains(first, "mj-carousel-aaaaaaaaaaaaaaaa") {
		t.Error("the carousel does not use the first generated ID")
	}
	if !strings.Contains(first, `id="bbbbbbbbbbbbbbbb"`) || !strings.Contains(first, `for="bbbbbbbbbbbbbbbb"`) {
		t.Error("the navbar checkbox does not use the second generated ID")
	}
	if ids := referenceIDGenerator(first); ids() != "aaaaaaaaaaaaaaaa" || ids() != "bbbbbbbbbbbbbbbb" {
		t.Error("referenceIDGenerator does not read the IDs back in order of first appearance")
	}
}

func TestRandomIDsWithoutGenerator(t *testing.T) {
	first, err := Render(carouselAndNavbar)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(carouselAndNavbar)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("two renders without a generator produced the same IDs")
	}
}

func TestNavbarWithoutHamburgerDoesNotShiftIDs(t *testing.T) {
	const twoNavbars = `<mjml><mj-body><mj-section><mj-column>
<mj-navbar><mj-navbar-link href="/a">A</mj-navbar-link></mj-navbar>
<mj-navbar hamburger="hamburger"><mj-navbar-link href="/b">B</mj-navbar-link></mj-navbar>
</mj-column></mj-section></mj-body></mjml>`
	html, err := Render(twoNavbars, WithIDGenerator(sequentialIDGenerator("aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `id="aaaaaaaaaaaaaaaa"`) || strings.Contains(html, "bbbbbbbbbbbbbbbb") {
		t.Error("the navbar without hamburger took the first ID from the generator")
	}
}
