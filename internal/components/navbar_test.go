package components

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

func TestNavbarRandomIDs(t *testing.T) {
	node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-navbar"}}
	opts := &options.RenderOpts{}

	seenIDs := make(map[string]bool)
	iterations := 10
	for i := range iterations {
		id := NewMJNavbarComponent(node, opts).generateCheckboxID()
		if !isElementID(id) {
			t.Errorf("Iteration %d: expected 16 lowercase hex digits, got %q", i, id)
		}
		if seenIDs[id] {
			t.Errorf("Iteration %d: got duplicate ID: %s", i, id)
		}
		seenIDs[id] = true
	}
}

func TestNavbarUsesIDGenerator(t *testing.T) {
	node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-navbar"}}
	calls := 0
	opts := &options.RenderOpts{IDGenerator: func() string {
		calls++
		return "00000000000000aa"
	}}

	if id := NewMJNavbarComponent(node, opts).generateCheckboxID(); id != "00000000000000aa" {
		t.Errorf("expected the generator's ID, got %s", id)
	}
	if calls != 1 {
		t.Errorf("expected one call to the generator, got %d", calls)
	}
}

func TestNavbarWithoutHamburgerTakesNoID(t *testing.T) {
	node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-navbar"}}
	calls := 0
	opts := &options.RenderOpts{IDGenerator: func() string {
		calls++
		return "00000000000000aa"
	}}
	var out strings.Builder
	if err := NewMJNavbarComponent(node, opts).Render(&out); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("a navbar without hamburger called the ID generator %d times", calls)
	}
}

func TestInvalidGeneratedIDIsReplaced(t *testing.T) {
	for _, id := range []string{"", "0123", "0123456789ABCDEF", `0123456789abcd"x`} {
		opts := &options.RenderOpts{IDGenerator: func() string { return id }}
		node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-carousel"}}
		c := NewMJCarouselComponent(node, opts)
		got := c.generateCarouselID()
		if !isElementID(got) {
			t.Errorf("generator value %q: got %q, want a random 16-hex-digit ID", id, got)
		}
		if again := c.generateCarouselID(); again != got {
			t.Errorf("generator value %q: the carousel ID changed from %q to %q", id, got, again)
		}
	}
}
