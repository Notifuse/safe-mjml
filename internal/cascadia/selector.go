package cascadia

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Matcher is the interface for basic selector functionality.
// Match returns whether a selector matches n.
type Matcher interface {
	Match(n *html.Node) bool
}

// Sel is the interface for all the functionality provided by selectors.
type Sel interface {
	Matcher
	Specificity() Specificity

	// Returns a CSS input compiling to this selector.
	String() string

	// Returns a pseudo-element, or an empty string.
	PseudoElement() string
}

// Parse parses a selector. Use `ParseWithPseudoElement`
// if you need support for pseudo-elements.
func Parse(sel string) (Sel, error) {
	p := &parser{s: sel}
	compiled, err := p.parseSelector()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// ParseWithPseudoElement parses a single selector,
// with support for pseudo-element.
func ParseWithPseudoElement(sel string) (Sel, error) {
	p := &parser{s: sel, acceptPseudoElements: true}
	compiled, err := p.parseSelector()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// ParseWithBudget is Parse, with a Budget for the work of matching the
// selector, which it shares with every selector parsed with the same Budget.
func ParseWithBudget(sel string, b *Budget) (Sel, error) {
	p := &parser{s: sel, budget: b}
	compiled, err := p.parseSelector()
	if err != nil {
		return nil, err
	}
	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}
	return compiled, nil
}

// ParseForJuice is ParseWithBudget in the css-select mode: the selector
// matches as css-select 5.2.2, juice's engine, matches it (cssselect.go).
func ParseForJuice(sel string, b *Budget) (Sel, error) {
	p := &parser{s: sel, budget: b, cssSelect: true}
	compiled, err := p.parseSelector()
	if err != nil {
		return nil, err
	}
	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}
	return compiled, nil
}

// ParseForCheerioXML is ParseForJuice as css-select matches with xmlMode,
// as cheerio's $() does on a document loaded with xmlMode (mjml-core's
// mj-html-attributes): tag and attribute names keep their case, and
// attribute values are compared with case unless the i flag says otherwise.
func ParseForCheerioXML(sel string, b *Budget) (Sel, error) {
	p := &parser{s: sel, budget: b, cssSelect: true, xml: true}
	compiled, err := p.parseSelector()
	if err != nil {
		return nil, err
	}
	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}
	return compiled, nil
}

// ParseGroup parses a selector, or a group of selectors separated by commas.
// Use `ParseGroupWithPseudoElements`
// if you need support for pseudo-elements.
func ParseGroup(sel string) (SelectorGroup, error) {
	p := &parser{s: sel}
	compiled, err := p.parseSelectorGroup()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// ParseGroupWithPseudoElements parses a selector, or a group of selectors separated by commas.
// It supports pseudo-elements.
func ParseGroupWithPseudoElements(sel string) (SelectorGroup, error) {
	p := &parser{s: sel, acceptPseudoElements: true}
	compiled, err := p.parseSelectorGroup()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// A Selector is a function which tells whether a node matches or not.
//
// This type is maintained for compatibility; I recommend using the newer and
// more idiomatic interfaces Sel and Matcher.
type Selector func(*html.Node) bool

// Compile parses a selector and returns, if successful, a Selector object
// that can be used to match against html.Node objects.
func Compile(sel string) (Selector, error) {
	compiled, err := ParseGroup(sel)
	if err != nil {
		return nil, err
	}

	return Selector(compiled.Match), nil
}

// MatchAll returns a slice of the nodes that match the selector,
// from n and its children.
func (s Selector) MatchAll(n *html.Node) []*html.Node {
	return s.matchAllInto(n, nil)
}

func (s Selector) matchAllInto(n *html.Node, storage []*html.Node) []*html.Node {
	if s(n) {
		storage = append(storage, n)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		storage = s.matchAllInto(child, storage)
	}

	return storage
}

func queryInto(n *html.Node, m Matcher, storage []*html.Node) []*html.Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if m.Match(child) {
			storage = append(storage, child)
		}
		storage = queryInto(child, m, storage)
	}

	return storage
}

// QueryAll returns a slice of all the nodes that match m, from the descendants
// of n.
func QueryAll(n *html.Node, m Matcher) []*html.Node {
	return queryInto(n, m, nil)
}

// Match returns true if the node matches the selector.
func (s Selector) Match(n *html.Node) bool {
	return s(n)
}

// MatchFirst returns the first node that matches s, from n and its children.
func (s Selector) MatchFirst(n *html.Node) *html.Node {
	if s.Match(n) {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		m := s.MatchFirst(c)
		if m != nil {
			return m
		}
	}
	return nil
}

// Query returns the first node that matches m, from the descendants of n.
// If none matches, it returns nil.
func Query(n *html.Node, m Matcher) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if m.Match(c) {
			return c
		}
		if matched := Query(c, m); matched != nil {
			return matched
		}
	}

	return nil
}

// Filter returns the nodes in nodes that match the selector.
func (s Selector) Filter(nodes []*html.Node) (result []*html.Node) {
	for _, n := range nodes {
		if s(n) {
			result = append(result, n)
		}
	}
	return result
}

type tagSelector struct {
	tag    string
	budget *Budget
}

// Matches elements with a given tag name.
func (t tagSelector) Match(n *html.Node) bool {
	return n.Type == html.ElementNode && t.budget.Spend(1) && n.Data == t.tag
}

func (c tagSelector) Specificity() Specificity {
	return Specificity{0, 0, 1}
}

func (c tagSelector) PseudoElement() string {
	return ""
}

type classSelector struct {
	class  string
	budget *Budget
}

// Matches elements by class attribute.
func (t classSelector) Match(n *html.Node) bool {
	return matchAttribute(t.budget, n, "class", func(s string) bool {
		return matchInclude(t.class, s, false)
	})
}

func (c classSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c classSelector) PseudoElement() string {
	return ""
}

type idSelector struct {
	id     string
	budget *Budget
}

// Matches elements by id attribute.
func (t idSelector) Match(n *html.Node) bool {
	return matchAttribute(t.budget, n, "id", func(s string) bool {
		return s == t.id
	})
}

func (c idSelector) Specificity() Specificity {
	return Specificity{1, 0, 0}
}

func (c idSelector) PseudoElement() string {
	return ""
}

type attrSelector struct {
	key, val, operation string
	regexp              *regexp.Regexp
	insensitive         bool
	flag                byte // 0, 'i' or, in the css-select mode, 's'
	budget              *Budget
}

// Matches elements by attribute value.
func (t attrSelector) Match(n *html.Node) bool {
	switch t.operation {
	case "":
		return matchAttribute(t.budget, n, t.key, func(string) bool { return true })
	case "=":
		return matchAttribute(t.budget, n, t.key, func(s string) bool { return matchInsensitiveValue(s, t.val, t.insensitive) })
	case "!=":
		return attributeNotEqualMatch(t.budget, t.key, t.val, n, t.insensitive)
	case "~=":
		// matches elements where the attribute named key is a whitespace-separated list that includes val.
		return matchAttribute(t.budget, n, t.key, func(s string) bool { return matchInclude(t.val, s, t.insensitive) })
	case "|=":
		return attributeDashMatch(t.budget, t.key, t.val, n, t.insensitive)
	case "^=":
		return attributePrefixMatch(t.budget, t.key, t.val, n, t.insensitive)
	case "$=":
		return attributeSuffixMatch(t.budget, t.key, t.val, n, t.insensitive)
	case "*=":
		return attributeSubstringMatch(t.budget, t.key, t.val, n, t.insensitive)
	case "#=":
		return attributeRegexMatch(t.budget, t.key, t.regexp, n)
	default:
		panic(fmt.Sprintf("unsuported operation : %s", t.operation))
	}
}

// matches elements where we ignore (or not) the case of the attribute value
// the user attribute is the value set by the user to match elements
// the real attribute is the attribute value found in the code parsed
func matchInsensitiveValue(userAttr string, realAttr string, ignoreCase bool) bool {
	if ignoreCase {
		return strings.EqualFold(userAttr, realAttr)
	}
	return userAttr == realAttr

}

// matches elements where the attribute named key satisifes the function f.
func matchAttribute(b *Budget, n *html.Node, key string, f func(string) bool) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if !b.Spend(1 + len(n.Attr)) {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == key && b.Spend(len(a.Val)/bytesPerStep) && f(a.Val) {
			return true
		}
	}
	return false
}

// attributeNotEqualMatch matches elements where
// the attribute named key does not have the value val.
func attributeNotEqualMatch(b *Budget, key, val string, n *html.Node, ignoreCase bool) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if !b.Spend(1 + len(n.Attr)) {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == key && b.Spend(len(a.Val)/bytesPerStep) && matchInsensitiveValue(a.Val, val, ignoreCase) {
			return false
		}
	}
	return true
}

// returns true if s is a whitespace-separated list that includes val.
func matchInclude(val string, s string, ignoreCase bool) bool {
	for s != "" {
		i := strings.IndexAny(s, " \t\r\n\f")
		if i == -1 {
			return matchInsensitiveValue(s, val, ignoreCase)
		}
		if matchInsensitiveValue(s[:i], val, ignoreCase) {
			return true
		}
		s = s[i+1:]
	}
	return false
}

// matches elements where the attribute named key equals val or starts with val plus a hyphen.
func attributeDashMatch(b *Budget, key, val string, n *html.Node, ignoreCase bool) bool {
	return matchAttribute(b, n, key,
		func(s string) bool {
			if matchInsensitiveValue(s, val, ignoreCase) {
				return true
			}
			if len(s) <= len(val) {
				return false
			}
			if matchInsensitiveValue(s[:len(val)], val, ignoreCase) && s[len(val)] == '-' {
				return true
			}
			return false
		})
}

// attributePrefixMatch returns a Selector that matches elements where
// the attribute named key starts with val.
func attributePrefixMatch(b *Budget, key, val string, n *html.Node, ignoreCase bool) bool {
	return matchAttribute(b, n, key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			if ignoreCase {
				if !b.Spend(len(s)) { // strings.ToLower
					return false
				}
				return strings.HasPrefix(strings.ToLower(s), strings.ToLower(val))
			}
			return strings.HasPrefix(s, val)
		})
}

// attributeSuffixMatch matches elements where
// the attribute named key ends with val.
func attributeSuffixMatch(b *Budget, key, val string, n *html.Node, ignoreCase bool) bool {
	return matchAttribute(b, n, key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			if ignoreCase {
				if !b.Spend(len(s)) { // strings.ToLower
					return false
				}
				return strings.HasSuffix(strings.ToLower(s), strings.ToLower(val))
			}
			return strings.HasSuffix(s, val)
		})
}

// attributeSubstringMatch matches nodes where
// the attribute named key contains val.
func attributeSubstringMatch(b *Budget, key, val string, n *html.Node, ignoreCase bool) bool {
	return matchAttribute(b, n, key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			if ignoreCase {
				if !b.Spend(len(s)) { // strings.ToLower
					return false
				}
				return strings.Contains(strings.ToLower(s), strings.ToLower(val))
			}
			return strings.Contains(s, val)
		})
}

// attributeRegexMatch  matches nodes where
// the attribute named key matches the regular expression rx
func attributeRegexMatch(b *Budget, key string, rx *regexp.Regexp, n *html.Node) bool {
	return matchAttribute(b, n, key,
		func(s string) bool {
			return rx.MatchString(s)
		})
}

func (c attrSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c attrSelector) PseudoElement() string {
	return ""
}

// see pseudo_classes.go for pseudo classes selectors

// on a static context, some selectors can't match anything
type neverMatchSelector struct {
	value string
}

func (s neverMatchSelector) Match(n *html.Node) bool {
	return false
}

func (s neverMatchSelector) Specificity() Specificity {
	return Specificity{0, 0, 0}
}

func (c neverMatchSelector) PseudoElement() string {
	return ""
}

type compoundSelector struct {
	selectors     []Sel
	pseudoElement string
}

// Matches elements if each sub-selectors matches.
func (t compoundSelector) Match(n *html.Node) bool {
	if len(t.selectors) == 0 {
		return n.Type == html.ElementNode
	}

	for _, sel := range t.selectors {
		if !sel.Match(n) {
			return false
		}
	}
	return true
}

func (s compoundSelector) Specificity() Specificity {
	var out Specificity
	for _, sel := range s.selectors {
		out = out.Add(sel.Specificity())
	}
	if s.pseudoElement != "" {
		// https://drafts.csswg.org/selectors-3/#specificity
		out = out.Add(Specificity{0, 0, 1})
	}
	return out
}

func (c compoundSelector) PseudoElement() string {
	return c.pseudoElement
}

type combinedSelector struct {
	first      Sel
	combinator byte
	second     Sel
	budget     *Budget
}

func (t combinedSelector) Match(n *html.Node) bool {
	if t.first == nil {
		return false // maybe we should panic
	}
	if t.combinator == 0 {
		return t.first.Match(n)
	}
	return t.status(n) == matched
}

// matchStatus is the result of matching a complex selector on an element,
// with what its failure says about other candidates. Matching right to left,
// a combinator tries the left side on each candidate (each ancestor for a
// descendant combinator, each earlier sibling for a general sibling one);
// without the failure kinds, ".x div div div div" walks every path of
// ancestors of a deep tree, a time in its depth to the power of the number of
// combinators. These are the kinds of WebKit's and Servo's selector matching
// (SelectorMatchingResult in Servo's selectors crate): the walk stops as soon
// as no other candidate can match.
type matchStatus uint8

const (
	matched matchStatus = iota
	// The rightmost compound failed: another earlier sibling may match.
	failedTryLaterSibling
	// A sibling combinator ran out of siblings: another ancestor may match,
	// but no other sibling.
	failedTryDescendant
	// A descendant or child combinator ran out of ancestors: no other
	// candidate can match, as each has fewer ancestors.
	failedGlobally
)

func statusOf(s Sel, n *html.Node) matchStatus {
	if c, ok := s.(combinedSelector); ok && c.combinator != 0 && c.first != nil {
		return c.status(n)
	}
	if s.Match(n) {
		return matched
	}
	return failedTryLaterSibling
}

// status matches t on n, trying the candidates of its combinator in order
// (see candidate), and returns as soon as the result is known for every
// candidate.
func (t combinedSelector) status(n *html.Node) matchStatus {
	if !t.second.Match(n) {
		return failedTryLaterSibling
	}
	notFound := failedGlobally
	if t.combinator == '+' || t.combinator == '~' {
		notFound = failedTryDescendant
	}
	for e := t.candidate(n); ; e = t.candidate(e) {
		if e == nil {
			return notFound
		}
		if !t.budget.Spend(1) {
			return failedGlobally
		}
		r := statusOf(t.first, e)
		switch {
		case r == matched, r == failedGlobally, t.combinator == '+':
			return r
		case t.combinator == '>':
			return failedTryDescendant
		case t.combinator == '~' && r == failedTryDescendant:
			return r
		}
		// A descendant combinator tries the next ancestor; a general
		// sibling combinator, the next earlier sibling.
	}
}

// candidate returns the node after e that t's combinator tries its left
// side on, or nil, as cascadia's descendant, child and sibling matching did:
// the parent for a descendant or child combinator (up to the document node,
// which matches nothing), the previous sibling for a general sibling one, and
// the previous sibling that is not text or a comment for an adjacent one. A
// child or adjacent combinator tries one candidate only: status returns after
// it.
func (t combinedSelector) candidate(e *html.Node) *html.Node {
	switch t.combinator {
	case ' ', '>':
		return e.Parent
	case '~':
		return e.PrevSibling
	case '+':
		for p := e.PrevSibling; p != nil; p = p.PrevSibling {
			if p.Type != html.TextNode && p.Type != html.CommentNode {
				return p
			}
		}
	}
	return nil
}

func (s combinedSelector) Specificity() Specificity {
	spec := s.first.Specificity()
	if s.second != nil {
		spec = spec.Add(s.second.Specificity())
	}
	return spec
}

// on combinedSelector, a pseudo-element only makes sens on the last
// selector, although others increase specificity.
func (c combinedSelector) PseudoElement() string {
	if c.second == nil {
		return ""
	}
	return c.second.PseudoElement()
}

// A SelectorGroup is a list of selectors, which matches if any of the
// individual selectors matches.
type SelectorGroup []Sel

// Match returns true if the node matches one of the single selectors.
func (s SelectorGroup) Match(n *html.Node) bool {
	for _, sel := range s {
		if sel.Match(n) {
			return true
		}
	}
	return false
}
