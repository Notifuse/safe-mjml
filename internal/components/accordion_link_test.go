package components

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestAccordionLinksElementsAddedToIt checks that adding an element to an
// accordion makes the accordion its parent, so the element takes the
// accordion's border, and that the element renders the title it was given.
func TestAccordionLinksElementsAddedToIt(t *testing.T) {
	opts := &options.RenderOpts{}
	node := func(tag string) *parser.MJMLNode {
		return &parser.MJMLNode{XMLName: xml.Name{Local: tag}}
	}
	accordion := NewMJAccordionComponent(node("mj-accordion"), opts)
	element := NewMJAccordionElementComponent(node("mj-accordion-element"), opts)
	// A tree assembled by hand sets each context, as the tree builder does.
	element.SetContext(ChildContextOf(accordion, 1, 0))
	addChildren(t, element, NewMJAccordionTitleComponent(node("mj-accordion-title"), opts))
	addChildren(t, accordion, element)

	var out strings.Builder
	if err := accordion.Render(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "border-bottom:2px solid black") {
		t.Error("the element does not take the accordion's border: it is not linked to its accordion")
	}
	if !strings.Contains(out.String(), "mj-accordion-title") {
		t.Error("the element does not render the title added to it")
	}
}

func TestHeroHeightWithoutNumberIsKept(t *testing.T) {
	hero := NewMJHeroComponent(&parser.MJMLNode{XMLName: xml.Name{Local: "mj-hero"}}, &options.RenderOpts{})
	if got := hero.calculateEffectiveHeight("abc"); got != "abc" {
		t.Errorf("calculateEffectiveHeight(\"abc\") = %q, want it kept as written", got)
	}
	if got := hero.calculateEffectiveHeight("300px"); got != "300px" {
		t.Errorf("calculateEffectiveHeight(\"300px\") = %q, want 300px", got)
	}
}

func addChildren(t *testing.T, parent Component, children ...Component) {
	t.Helper()
	for _, child := range children {
		if err := parent.AddChild(child); err != nil {
			t.Fatal(err)
		}
	}
}
