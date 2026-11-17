package mjml

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// These cases are not valid MJML, but MJML's default soft validation renders
// them, so safe-mjml must render them too, without failing or adding output for
// what is not rendered.

func TestAccordionElementOutsideAccordion(t *testing.T) {
	for _, parent := range []string{"mj-column", "mj-hero"} {
		t.Run(parent, func(t *testing.T) {
			element := `<mj-accordion-element><mj-accordion-title>Question</mj-accordion-title><mj-accordion-text>Answer</mj-accordion-text></mj-accordion-element>`
			src := `<mjml><mj-body><mj-section><mj-column>` + element + `</mj-column></mj-section></mj-body></mjml>`
			if parent == "mj-hero" {
				src = `<mjml><mj-body><mj-hero>` + element + `</mj-hero></mj-body></mjml>`
			}
			html, err := Render(src)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(html, "Question") || !strings.Contains(html, "Answer") {
				t.Error("the accordion element is not rendered")
			}
		})
	}
}

func TestCarouselInsideCarouselIsNotRendered(t *testing.T) {
	src := `<mjml><mj-body><mj-section><mj-column><mj-carousel>
<mj-carousel><mj-carousel-image src="https://example.com/inner.png" /></mj-carousel>
<mj-carousel-image src="https://example.com/outer.png" />
</mj-carousel></mj-column></mj-section></mj-body></mjml>`
	calls := 0
	html, err := Render(src, WithIDGenerator(func() string {
		calls++
		return "aaaaaaaaaaaaaaaa"
	}))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("the ID generator was called %d times, want once: only the outer carousel is rendered", calls)
	}
	if strings.Contains(html, "inner.png") {
		t.Error("the inner carousel is rendered")
	}
}

func TestComponentsNotRenderedAddNoHeadStyles(t *testing.T) {
	withImage := `<mjml><mj-body><mj-section><mj-column><mj-carousel>
<mj-carousel-image src="https://example.com/a.png" /><mj-image src="https://example.com/b.png" />
</mj-carousel></mj-column></mj-section></mj-body></mjml>`
	without := strings.Replace(withImage, `<mj-image src="https://example.com/b.png" />`, "", 1)
	got, err := Render(withImage, WithIDGenerator(sequentialIDGenerator()))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Render(without, WithIDGenerator(sequentialIDGenerator()))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Error("an mj-image the carousel does not render changes the output")
	}
}

func TestNestedMJMLIsIgnored(t *testing.T) {
	doc := func(nested string) string {
		return `<mjml lang="fr"><mj-head><mj-style inline="inline">.red { color: red; }</mj-style></mj-head><mj-body>
<mj-hero>` + nested + `<mj-text css-class="red">Hello</mj-text></mj-hero></mj-body></mjml>`
	}
	got, err := Render(doc(`<mjml><mj-body></mj-body></mjml>`))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Render(doc(""))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Error("a nested mjml element changes the output of the document")
	}
}

// TestAccordionElementOutsideAccordionIsStable checks that an element outside
// any mj-accordion resolves its attributes the same way in every pass and
// every render: like MJML, it takes nothing from mj-attributes for
// mj-accordion, since no accordion passes them down.
func TestAccordionElementOutsideAccordionIsStable(t *testing.T) {
	src, err := os.ReadFile("testdata/mj-accordion-element-standalone.mjml")
	if err != nil {
		t.Fatal(err)
	}
	ast, err := parser.ParseMJML(string(src))
	if err != nil {
		t.Fatal(err)
	}
	comp, err := newComponentTree(ast)
	if err != nil {
		t.Fatal(err)
	}
	first, err := renderComponentString(comp)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderComponentString(comp)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("two renders of the same tree differ")
	}
	if strings.Contains(first, "Roboto") || strings.Contains(first, "50px") {
		t.Error("the mj-attributes of mj-accordion apply to an element outside any accordion")
	}
}

// TestFragmentRoots checks that a template whose root is not mjml is an
// error at its root, as MJML throws "Malformed MJML. Check that your
// structure is correct and enclosed in <mjml> tags." for it. This test
// checked, when the low-level AST API was public, that such a fragment
// rendered its children; that API is gone, and Pierre decided that v1
// follows MJML (checked with MJML 5.4.1 on these three templates), and then
// that Parse refuses them already, so that a Document always renders.
func TestFragmentRoots(t *testing.T) {
	for _, template := range []string{
		`<mj-section><mj-column><mj-text>Fragment text</mj-text></mj-column></mj-section>`,
		`<mj-head></mj-head>`,
		`<mj-unknown></mj-unknown>`,
	} {
		ast, err := parser.ParseMJML(template)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := newComponentTree(ast); err == nil || !strings.Contains(err.Error(), "malformed MJML") {
			t.Errorf("%s: newComponentTree gives %v, want the malformed MJML error", template, err)
		}
		doc, err := Parse(template)
		var mjmlErr *Error
		if doc != nil || !errors.As(err, &mjmlErr) || len(mjmlErr.Details) != 1 || mjmlErr.Details[0].Line != 1 ||
			!strings.Contains(mjmlErr.Details[0].Message, "the root element is "+ast.GetTagName()+", not mjml") {
			t.Errorf("%s: Parse gives %v, %v, want no Document and the malformed MJML error at line 1", template, doc, err)
		}
		if _, err := Render(template); err == nil {
			t.Errorf("%s: Render gives no error", template)
		}
	}
}

func TestDocumentRootChildren(t *testing.T) {
	ast, err := parser.ParseMJML(`<mjml><mj-head><mj-title>T</mj-title></mj-head><mj-body><mj-section><mj-column><mj-text>Hi</mj-text></mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	comp, err := newComponentTree(ast)
	if err != nil {
		t.Fatal(err)
	}
	var tags []string
	components.Walk(comp, func(c components.Component) bool {
		tags = append(tags, c.GetTagName())
		return true
	})
	want := []string{"mjml", "mj-head", "mj-title", "mj-body", "mj-section", "mj-column", "mj-text"}
	if strings.Join(tags, " ") != strings.Join(want, " ") {
		t.Errorf("Walk from the document root saw %v, want %v", tags, want)
	}

	if err := comp.AddChild(components.NewMJRawComponent(ast, nil)); err == nil {
		t.Error("AddChild on the mjml root returns no error")
	}
}

func TestAccordionElementOutsideAccordionKeepsItsBorder(t *testing.T) {
	html, err := Render(`<mjml><mj-body><mj-section><mj-column>
<mj-accordion-element border="4px dotted green"><mj-accordion-title>Q</mj-accordion-title><mj-accordion-text>A</mj-accordion-text></mj-accordion-element>
</mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(html, "border-bottom:4px dotted green") != 2 {
		t.Error("an element outside any accordion does not keep its own border on its title and text")
	}
}

// TestHugeLengthsDoNotOverflow checks that a length too large for an int, as
// a user-written template may hold, gives neither an overflowed width nor a
// child wider than its parent.
func TestHugeLengthsDoNotOverflow(t *testing.T) {
	html, err := Render(`<mjml><mj-body><mj-section padding="0 99999999999999999999px">
<mj-column><mj-image src="https://example.com/a.png" /><mj-divider /></mj-column></mj-section>
<mj-hero height="99999999999999999999px"><mj-text>Hi</mj-text></mj-hero></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`width="(-?\d+)(?:px)?"`).FindAllStringSubmatch(html, -1) {
		if n, _ := strconv.Atoi(m[1]); n < 0 || n > 600 {
			t.Errorf("width=%q is negative or wider than the body", m[1])
		}
	}

	html, err = Render(`<mjml><mj-body width="99999999999999999999px"><mj-section><mj-column><mj-image src="https://example.com/a.png" /></mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`width:(-?\d+)px`).FindAllStringSubmatch(html, -1) {
		if n, err := strconv.ParseInt(m[1], 10, 64); err != nil || n < 0 || n > 1<<31-1 {
			t.Errorf("width:%spx overflowed", m[1])
		}
	}
	for _, m := range regexp.MustCompile(`height="(-?\d+)"`).FindAllStringSubmatch(html, -1) {
		if n, err := strconv.ParseInt(m[1], 10, 64); err != nil || n < 0 || n > 1<<31-1 {
			t.Errorf("height=%q overflowed", m[1])
		}
	}
}
