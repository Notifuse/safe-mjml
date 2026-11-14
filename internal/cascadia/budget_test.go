package cascadia

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestBudgetCountsEveryWalk matches selectors whose walks are long on trees
// made for them, and requires the Budget to count at least the nodes and
// attributes each walk visits: a loop that does not take its steps would let
// a template walk without a limit. Within the Budget, the matches are those
// of Parse; past it, Exceeded reports it.
func TestBudgetCountsEveryWalk(t *testing.T) {
	const n = 400
	element := func(tag string, attrs ...html.Attribute) *html.Node {
		return &html.Node{Type: html.ElementNode, Data: tag, Attr: attrs}
	}
	deep := func(tag string, attrs ...html.Attribute) *html.Node {
		doc := &html.Node{Type: html.DocumentNode}
		for e, i := doc, 0; i < n; i++ {
			c := element(tag, attrs...)
			e.AppendChild(c)
			e = c
		}
		return doc
	}
	// deepWithText has a text node next to each div: :has visits it too,
	// though no selector inside it tests text.
	deepWithText := func() *html.Node {
		doc := &html.Node{Type: html.DocumentNode}
		for e, i := doc, 0; i < n; i++ {
			c := element("div")
			e.AppendChild(&html.Node{Type: html.TextNode, Data: "t"})
			e.AppendChild(c)
			e = c
		}
		return doc
	}
	wide := func() *html.Node {
		doc := &html.Node{Type: html.DocumentNode}
		row := element("div")
		doc.AppendChild(row)
		for range n {
			row.AppendChild(element("p"))
		}
		return doc
	}
	manyAttributes := func() *html.Node {
		doc := &html.Node{Type: html.DocumentNode}
		var attrs []html.Attribute
		for i := range n {
			attrs = append(attrs, html.Attribute{Key: fmt.Sprintf("data-a%d", i), Val: "v"})
		}
		for range n {
			doc.AppendChild(element("p", attrs...))
		}
		return doc
	}
	// longValues: n elements whose value is 100 steps of bytes long, and
	// whose text is as long.
	longValues := func() *html.Node {
		doc := &html.Node{Type: html.DocumentNode}
		row := element("div")
		doc.AppendChild(row)
		for range n {
			p := element("p", html.Attribute{Key: "data-v", Val: strings.Repeat("y", 100*bytesPerStep)})
			p.AppendChild(&html.Node{Type: html.TextNode, Data: strings.Repeat("y", 100*bytesPerStep)})
			row.AppendChild(p)
		}
		return doc
	}
	half := n * (n - 1) / 2 // the nodes the walks of all elements visit, at least
	for _, tc := range []struct {
		name string
		doc  *html.Node
		sel  string
		work int
	}{
		{"descendant combinator", deep("div"), ".absent div", half},
		{"general sibling combinator", wide(), ".absent ~ p", half},
		{":has", deepWithText(), "div:has(.absent)", 2 * half},
		{":contains", deep("div"), "div:contains(absent)", half},
		{":nth-last-child", wide(), "p:nth-last-child(2n+1)", n * n},
		{"attributes", manyAttributes(), "[data-absent]", n * n},
		{":lang", deep("div"), "div:lang(fr)", half},
		{"bytes of attribute values", longValues(), "p[data-v^=x]", n * 100},
		{"bytes of text", longValues(), "p:contains(x)", n * 100},
		{":not()", wide(), "p:not(:not(:not(:not(.absent))))", 4 * n},
		{"lowering attribute values", longValues(), "p[data-v*=x i]", n * 100 * bytesPerStep},
		{"lowering text", longValues(), "p:icontains(x)", n * 100 * bytesPerStep},
	} {
		plain, err := Parse(tc.sel)
		if err != nil && tc.name != "lowering text" {
			t.Fatal(err)
		}
		for mode, parse := range map[string]func(string, *Budget) (Sel, error){"cascadia": ParseWithBudget, "css-select": ParseForJuice} {
			if mode == "css-select" && tc.name == ":lang" || mode == "cascadia" && tc.name == "lowering text" {
				continue // :lang is not css-select's (juicer refuses it), :icontains not cascadia's
			}
			b := NewBudget(1 << 40)
			sel, err := parse(tc.sel, b)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "cascadia" {
				want := matchSet(tc.doc, plain.Match)
				if got := matchSet(tc.doc, sel.Match); !sameNodes(got, want) {
					t.Errorf("%s: %d matches, %d without a budget", tc.name, len(got), len(want))
				}
			} else {
				matchSet(tc.doc, sel.Match)
			}
			if used := 1<<40 - b.left; used < int64(tc.work) {
				t.Errorf("%s, %s: %d steps counted for at least %d nodes, attributes or steps of bytes read", tc.name, mode, used, tc.work)
			}
			small := NewBudget(int64(tc.work / 10))
			sel, _ = parse(tc.sel, small)
			matchSet(tc.doc, sel.Match)
			if !small.Exceeded() {
				t.Errorf("%s, %s: a tenth of its work is not exceeded", tc.name, mode)
			}
		}
	}
	if b := (*Budget)(nil); !b.Spend(1<<30) || b.Exceeded() {
		t.Error("a nil Budget has a limit")
	}
}

// Every selector parsed with a Budget shares it, the arguments of :is(),
// :not() and :has() included, and stops when it is spent.
func TestBudgetIsShared(t *testing.T) {
	doc, err := html.Parse(strings.NewReader("<body>" + strings.Repeat("<p class=a><span>x</span></p>", 50) + "</body>"))
	if err != nil {
		t.Fatal(err)
	}
	b := NewBudget(1 << 40)
	for _, s := range []string{"p:not(.b)", "p:has(span)", "body > p span"} {
		sel, err := ParseWithBudget(s, b)
		if err != nil {
			t.Fatal(err)
		}
		before := b.left
		matchSet(doc, sel.Match)
		if b.left == before {
			t.Errorf("%s took no step of the shared Budget", s)
		}
	}
}
