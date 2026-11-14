package cascadia

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

// naiveMatch is cascadia's matching before the failure kinds (matchStatus):
// every candidate of every combinator is tried. It is the oracle of
// TestCombinatorsMatchAsBefore.
func naiveMatch(s Sel, n *html.Node) bool {
	t, ok := s.(combinedSelector)
	if !ok || t.combinator == 0 {
		return s.Match(n)
	}
	if !t.second.Match(n) {
		return false
	}
	switch t.combinator {
	case ' ':
		for p := n.Parent; p != nil; p = p.Parent {
			if naiveMatch(t.first, p) {
				return true
			}
		}
	case '>':
		return n.Parent != nil && naiveMatch(t.first, n.Parent)
	case '+':
		for p := n.PrevSibling; p != nil; p = p.PrevSibling {
			if p.Type == html.TextNode || p.Type == html.CommentNode {
				continue
			}
			return naiveMatch(t.first, p)
		}
	case '~':
		for p := n.PrevSibling; p != nil; p = p.PrevSibling {
			if naiveMatch(t.first, p) {
				return true
			}
		}
	}
	return false
}

// matchSet returns the nodes of doc that match, in document order.
func matchSet(doc *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func sameNodes(a, b []*html.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestCombinatorsMatchAsBefore requires the matching that stops early
// (matchStatus) to match the nodes the naive one matches: on the W3C
// selectors over their document, and on random chains of the four
// combinators over random trees, with text and comments between elements.
func TestCombinatorsMatchAsBefore(t *testing.T) {
	doc := parseReference("test_resources/content.xhtml")
	compared := 0
	for _, test := range loadValidSelectors(t) {
		sels, err := ParseGroup(test.Selector)
		if err != nil {
			continue // a pseudo-element
		}
		for _, sel := range sels {
			got := matchSet(doc, sel.Match)
			want := matchSet(doc, func(n *html.Node) bool { return naiveMatch(sel, n) })
			if !sameNodes(got, want) {
				t.Errorf("%s: %d nodes, %d before", test.Selector, len(got), len(want))
			}
			compared++
		}
	}
	if compared < 100 {
		t.Fatalf("only %d W3C selectors compared", compared)
	}

	r := rand.New(rand.NewSource(1))
	tags := []string{"div", "p", "span"}
	classes := []string{"a", "b", "c"}
	compound := func() string {
		var b strings.Builder
		if r.Intn(4) == 0 {
			b.WriteString("*")
		} else {
			b.WriteString(tags[r.Intn(len(tags))])
		}
		if r.Intn(2) == 0 {
			b.WriteString("." + classes[r.Intn(len(classes))])
		}
		switch r.Intn(6) {
		case 0:
			b.WriteString(":first-child")
		case 1:
			b.WriteString(":not(." + classes[r.Intn(len(classes))] + ")")
		case 2:
			b.WriteString(":has(span." + classes[r.Intn(len(classes))] + ")")
		}
		return b.String()
	}
	combinators := []string{" ", " > ", " + ", " ~ "}
	var tree func(depth int) string
	tree = func(depth int) string {
		var b strings.Builder
		for range 1 + r.Intn(4) {
			switch r.Intn(5) {
			case 0:
				b.WriteString("t")
			case 1:
				b.WriteString("<!--c-->")
			}
			tag := tags[r.Intn(len(tags))]
			fmt.Fprintf(&b, `<%s class="%s">`, tag, classes[r.Intn(len(classes))])
			if depth > 0 && r.Intn(4) != 0 {
				b.WriteString(tree(depth - 1))
			}
			fmt.Fprintf(&b, "</%s>", tag)
		}
		return b.String()
	}
	matches := 0
	const cases = 4000
	for n := range cases {
		doc, err := html.Parse(strings.NewReader("<body>" + tree(6) + "</body>"))
		if err != nil {
			t.Fatal(err)
		}
		var text strings.Builder
		text.WriteString(compound())
		for range 1 + r.Intn(5) {
			text.WriteString(combinators[r.Intn(len(combinators))])
			text.WriteString(compound())
		}
		sel, err := Parse(text.String())
		if err != nil {
			t.Fatalf("%s: %v", text.String(), err)
		}
		got := matchSet(doc, sel.Match)
		want := matchSet(doc, func(n *html.Node) bool { return naiveMatch(sel, n) })
		if !sameNodes(got, want) {
			t.Fatalf("case %d, %s: %d nodes, %d before", n, text.String(), len(got), len(want))
		}
		if len(want) > 0 {
			matches++
		}
	}
	// The comparison means something only if many selectors match.
	if matches < cases/5 {
		t.Errorf("only %d of %d random selectors match a node", matches, cases)
	}
	t.Logf("%d of %d random selectors match a node", matches, cases)
}

// A chain of descendant or sibling combinators on a deep or wide tree stops
// at the first candidate that rules out the others: tried naively, these
// take a time in the depth to the power of 7 (hours).
func TestCombinatorChainsStopEarly(t *testing.T) {
	// Built by hand: html.Parse stops at 512 open elements.
	const size = 2000
	element := func(tag string) *html.Node { return &html.Node{Type: html.ElementNode, Data: tag} }
	deep := &html.Node{Type: html.DocumentNode}
	for e, i := deep, 0; i < size; i++ {
		c := element("div")
		e.AppendChild(c)
		e = c
	}
	wide := &html.Node{Type: html.DocumentNode}
	row := element("div")
	wide.AppendChild(row)
	for range size {
		row.AppendChild(element("p"))
	}
	for _, tc := range []struct {
		doc *html.Node
		sel string
	}{
		{deep, ".absent div div div div div div"},
		{deep, "div div div div div div .absent"},
		{deep, ".absent > div div > div div div div"},
		{wide, ".absent ~ p ~ p ~ p ~ p ~ p ~ p"},
		{wide, "div .absent ~ p ~ p ~ p ~ p ~ p"},
		{wide, ".absent p ~ p ~ p ~ p ~ p ~ p"},
	} {
		sel, err := Parse(tc.sel)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		if got := matchSet(tc.doc, sel.Match); len(got) != 0 {
			t.Errorf("%s matches %d nodes", tc.sel, len(got))
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%s: %v", tc.sel, d)
		}
		t.Logf("%s: %v", tc.sel, time.Since(start))
	}
}
