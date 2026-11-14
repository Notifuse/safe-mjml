package cascadia

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// This file implements the pseudo classes selectors,
// which share the implementation of PseudoElement() and Specificity()

type abstractPseudoClass struct {
	budget *Budget // see Budget; nil has no limit
}

func (s abstractPseudoClass) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c abstractPseudoClass) PseudoElement() string {
	return ""
}

type relativePseudoClassSelector struct {
	name   string // one of "not", "has", "haschild"
	match  SelectorGroup
	budget *Budget
}

func (s relativePseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if !s.budget.Spend(1) {
		return false
	}
	switch s.name {
	case "not":
		// matches elements that do not match a.
		return !s.match.Match(n)
	case "has":
		//  matches elements with any descendant that matches a.
		return hasDescendantMatch(s.budget, n, s.match)
	case "haschild":
		// matches elements with a child that matches a.
		return hasChildMatch(s.budget, n, s.match)
	default:
		panic(fmt.Sprintf("unsupported relative pseudo class selector : %s", s.name))
	}
}

// hasChildMatch returns whether n has any child that matches a.
func hasChildMatch(b *Budget, n *html.Node, a SelectorGroup) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !b.Spend(1) {
			return false
		}
		if a.Match(c) {
			return true
		}
	}
	return false
}

// hasDescendantMatch performs a depth-first search of n's descendants,
// testing whether any of them match a. It returns true as soon as a match is
// found, or false if no match is found.
func hasDescendantMatch(b *Budget, n *html.Node, a SelectorGroup) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !b.Spend(1) {
			return false
		}
		if a.Match(c) || (c.Type == html.ElementNode && hasDescendantMatch(b, c, a)) {
			return true
		}
	}
	return false
}

// Specificity returns the specificity of the most specific selectors
// in the pseudo-class arguments.
// See https://www.w3.org/TR/selectors/#specificity-rules
func (s relativePseudoClassSelector) Specificity() Specificity {
	var max Specificity
	for _, sel := range s.match {
		newSpe := sel.Specificity()
		if max.Less(newSpe) {
			max = newSpe
		}
	}
	return max
}

func (c relativePseudoClassSelector) PseudoElement() string {
	return ""
}

type containsPseudoClassSelector struct {
	abstractPseudoClass
	value string
	own   bool
}

func (s containsPseudoClassSelector) Match(n *html.Node) bool {
	var text string
	if s.own {
		// matches nodes that directly contain the given text
		text = nodeOwnText(s.budget, n)
	} else {
		// matches nodes that contain the given text.
		text = nodeText(s.budget, n)
	}
	if !s.budget.Spend(len(text)) { // strings.ToLower
		return false
	}
	return strings.Contains(strings.ToLower(text), s.value)
}

type regexpPseudoClassSelector struct {
	abstractPseudoClass
	regexp *regexp.Regexp
	own    bool
}

func (s regexpPseudoClassSelector) Match(n *html.Node) bool {
	var text string
	if s.own {
		// matches nodes whose text directly matches the specified regular expression
		text = nodeOwnText(s.budget, n)
	} else {
		// matches nodes whose text matches the specified regular expression
		text = nodeText(s.budget, n)
	}
	return s.regexp.MatchString(text)
}

// writeNodeText writes the text contained in n and its descendants to buf.
func writeNodeText(b *Budget, n *html.Node, buf *bytes.Buffer) {
	if !b.Spend(1 + len(n.Data)/bytesPerStep) {
		return
	}
	switch n.Type {
	case html.TextNode:
		buf.WriteString(n.Data)
	case html.ElementNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeNodeText(b, c, buf)
		}
	}
}

// nodeText returns the text contained in n and its descendants.
func nodeText(b *Budget, n *html.Node) string {
	var buf bytes.Buffer
	writeNodeText(b, n, &buf)
	return buf.String()
}

// nodeOwnText returns the contents of the text nodes that are direct
// children of n.
func nodeOwnText(b *Budget, n *html.Node) string {
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil && b.Spend(1+len(c.Data)/bytesPerStep); c = c.NextSibling {
		if c.Type == html.TextNode {
			buf.WriteString(c.Data)
		}
	}
	return buf.String()
}

type nthPseudoClassSelector struct {
	abstractPseudoClass
	a, b         int
	last, ofType bool
	// parentElement: the css-select mode's test for a formula true at
	// every position, which nth-check compiles to "has a parent element".
	parentElement bool
}

func (s nthPseudoClassSelector) Match(n *html.Node) bool {
	if s.parentElement {
		return n.Type == html.ElementNode && n.Parent != nil && n.Parent.Type == html.ElementNode
	}
	if s.a == 0 {
		if s.last {
			return simpleNthLastChildMatch(s.budget, s.b, s.ofType, n)
		} else {
			return simpleNthChildMatch(s.budget, s.b, s.ofType, n)
		}
	}
	return nthChildMatch(s.budget, s.a, s.b, s.last, s.ofType, n)
}

// nthChildMatch implements :nth-child(an+b).
// If last is true, implements :nth-last-child instead.
// If ofType is true, implements :nth-of-type instead.
func nthChildMatch(budget *Budget, a, b int, last, ofType bool, n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	parent := n.Parent
	if parent == nil {
		return false
	}

	i := -1
	count := 0
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if !budget.Spend(1) {
			return false
		}
		if (c.Type != html.ElementNode) || (ofType && c.Data != n.Data) {
			continue
		}
		count++
		if c == n {
			i = count
			if !last {
				break
			}
		}
	}

	if i == -1 {
		// This shouldn't happen, since n should always be one of its parent's children.
		return false
	}

	if last {
		i = count - i + 1
	}

	i -= b
	if a == 0 {
		return i == 0
	}

	return i%a == 0 && i/a >= 0
}

// simpleNthChildMatch implements :nth-child(b).
// If ofType is true, implements :nth-of-type instead.
func simpleNthChildMatch(budget *Budget, b int, ofType bool, n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	parent := n.Parent
	if parent == nil {
		return false
	}

	count := 0
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if !budget.Spend(1) {
			return false
		}
		if c.Type != html.ElementNode || (ofType && c.Data != n.Data) {
			continue
		}
		count++
		if c == n {
			return count == b
		}
		if count >= b {
			return false
		}
	}
	return false
}

// simpleNthLastChildMatch implements :nth-last-child(b).
// If ofType is true, implements :nth-last-of-type instead.
func simpleNthLastChildMatch(budget *Budget, b int, ofType bool, n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	parent := n.Parent
	if parent == nil {
		return false
	}

	count := 0
	for c := parent.LastChild; c != nil; c = c.PrevSibling {
		if !budget.Spend(1) {
			return false
		}
		if c.Type != html.ElementNode || (ofType && c.Data != n.Data) {
			continue
		}
		count++
		if c == n {
			return count == b
		}
		if count >= b {
			return false
		}
	}
	return false
}

type onlyChildPseudoClassSelector struct {
	abstractPseudoClass
	ofType bool
}

// Match implements :only-child.
// If `ofType` is true, it implements :only-of-type instead.
func (s onlyChildPseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	parent := n.Parent
	if parent == nil {
		return false
	}

	count := 0
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if !s.budget.Spend(1) {
			return false
		}
		if (c.Type != html.ElementNode) || (s.ofType && c.Data != n.Data) {
			continue
		}
		count++
		if count > 1 {
			return false
		}
	}

	return count == 1
}

type inputPseudoClassSelector struct {
	abstractPseudoClass
}

// Matches input, select, textarea and button elements.
func (s inputPseudoClassSelector) Match(n *html.Node) bool {
	return n.Type == html.ElementNode && (n.Data == "input" || n.Data == "select" || n.Data == "textarea" || n.Data == "button")
}

type emptyElementPseudoClassSelector struct {
	abstractPseudoClass
}

// Matches empty elements.
func (s emptyElementPseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !s.budget.Spend(1 + len(c.Data)/bytesPerStep) {
			return false
		}
		switch c.Type {
		case html.ElementNode:
			return false
		case html.TextNode:
			if strings.TrimSpace(c.Data) == "" {
				continue
			} else {
				return false
			}
		}
	}

	return true
}

type rootPseudoClassSelector struct {
	abstractPseudoClass
}

// Match implements :root
func (s rootPseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if n.Parent == nil {
		return false
	}
	return n.Parent.Type == html.DocumentNode
}

func hasAttr(b *Budget, n *html.Node, attr string) bool {
	return matchAttribute(b, n, attr, func(string) bool { return true })
}

type linkPseudoClassSelector struct {
	abstractPseudoClass
}

// Match implements :link
func (s linkPseudoClassSelector) Match(n *html.Node) bool {
	return (n.DataAtom == atom.A || n.DataAtom == atom.Area || n.DataAtom == atom.Link) && hasAttr(s.budget, n, "href")
}

type langPseudoClassSelector struct {
	abstractPseudoClass
	lang string
}

func (s langPseudoClassSelector) Match(n *html.Node) bool {
	own := matchAttribute(s.budget, n, "lang", func(val string) bool {
		return val == s.lang || strings.HasPrefix(val, s.lang+"-")
	})
	if n.Parent == nil {
		return own
	}
	return own || s.Match(n.Parent)
}

type enabledPseudoClassSelector struct {
	abstractPseudoClass
}

func (s enabledPseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.A, atom.Area, atom.Link:
		return hasAttr(s.budget, n, "href")
	case atom.Optgroup, atom.Menuitem, atom.Fieldset:
		return !hasAttr(s.budget, n, "disabled")
	case atom.Button, atom.Input, atom.Select, atom.Textarea, atom.Option:
		return !hasAttr(s.budget, n, "disabled") && !inDisabledFieldset(s.budget, n)
	}
	return false
}

type disabledPseudoClassSelector struct {
	abstractPseudoClass
}

func (s disabledPseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.Optgroup, atom.Menuitem, atom.Fieldset:
		return hasAttr(s.budget, n, "disabled")
	case atom.Button, atom.Input, atom.Select, atom.Textarea, atom.Option:
		return hasAttr(s.budget, n, "disabled") || inDisabledFieldset(s.budget, n)
	}
	return false
}

func hasLegendInPreviousSiblings(b *Budget, n *html.Node) bool {
	for s := n.PrevSibling; s != nil; s = s.PrevSibling {
		if !b.Spend(1) {
			return false
		}
		if s.DataAtom == atom.Legend {
			return true
		}
	}
	return false
}

func inDisabledFieldset(b *Budget, n *html.Node) bool {
	if !b.Spend(1) {
		return false
	}
	if n.Parent == nil {
		return false
	}
	if n.Parent.DataAtom == atom.Fieldset && hasAttr(b, n.Parent, "disabled") &&
		(n.DataAtom != atom.Legend || hasLegendInPreviousSiblings(b, n)) {
		return true
	}
	return inDisabledFieldset(b, n.Parent)
}

type checkedPseudoClassSelector struct {
	abstractPseudoClass
}

func (s checkedPseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.Input, atom.Menuitem:
		return hasAttr(s.budget, n, "checked") && matchAttribute(s.budget, n, "type", func(val string) bool {
			t := toLowerASCII(val)
			return t == "checkbox" || t == "radio"
		})
	case atom.Option:
		return hasAttr(s.budget, n, "selected")
	}
	return false
}
