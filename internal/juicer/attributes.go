package juicer

import (
	"bytes"
	"fmt"

	"golang.org/x/net/html"

	"github.com/Notifuse/safe-mjml/internal/cascadia"
	"github.com/Notifuse/safe-mjml/internal/juicer/htmlparser2"
	"github.com/Notifuse/safe-mjml/internal/juicer/selector"
)

// AttributeRule is a selector and the attributes to set, in order, on every
// element it matches: an mj-selector of mj-html-attributes.
type AttributeRule struct {
	Selector   string
	Attributes []html.Attribute
}

// SetAttributes does what mjml-core does with the mj-html-attributes of a
// template. It reads doc as cheerio reads it with xmlMode and without
// decoding entities (htmlparser2's XML mode), sets each attribute of each
// rule, in order, on every element its selector matches, as cheerio's
// $(selector) does (the selector engine juice 11 uses, Juice11), replacing
// the value of an attribute the element already has, and writes the document
// back as cheerio does in XML mode: an element without children closes
// itself. A selector cheerio cannot read sets nothing and is returned in
// invalid (MJML throws on it). Template tags are protected as Inline protects
// them (CodeBlocks). The options are Inline's; MaxMatchSteps bounds the
// matching.
func SetAttributes(doc string, rules []AttributeRule, opts ...Option) (out string, invalid []string, err error) {
	in, err := New(append(opts, CompatibilityMode(Juice11))...)
	if err != nil {
		return "", nil, err
	}
	o := in.opts
	o.xmlMode = true
	if err := checkSize([]byte(doc), &o); err != nil {
		return "", nil, err
	}
	src, blocks := encodeCodeBlocks([]byte(doc), o.codeBlocks)
	root, err := parseDocument(src, &o)
	if err != nil {
		return "", nil, err
	}
	if o.maxMatchSteps > 0 {
		o.budget = cascadia.NewBudget(o.maxMatchSteps)
	}
	for _, r := range rules {
		matchers, ok := attributeMatchers(r.Selector, &o)
		if !ok {
			invalid = append(invalid, r.Selector)
			continue
		}
		var matched []*html.Node
		htmlparser2.Traverse(root, func(n *html.Node) bool {
			if n.Type == html.ElementNode {
				for _, m := range matchers {
					if m.Match(n) {
						matched = append(matched, n)
						break
					}
				}
			}
			return !o.budget.Exceeded()
		}, func(*html.Node) {})
		if o.budget.Exceeded() {
			return "", nil, fmt.Errorf("%w: matching the selectors of mj-html-attributes takes more than %d steps", ErrLimitExceeded, o.maxMatchSteps)
		}
		for _, n := range matched {
			for _, a := range r.Attributes {
				setAttribute(n, a)
			}
		}
	}
	var buf bytes.Buffer
	htmlparser2.RenderXML(&buf, root)
	result, ok := decodeCodeBlocks(buf.Bytes(), blocks, o.budget)
	if !ok {
		return "", nil, fmt.Errorf("%w: writing the template tags back takes more than %d steps", ErrLimitExceeded, o.maxMatchSteps)
	}
	return string(result), invalid, nil
}

// attributeMatchers compiles a selector list as cheerio reads it, or reports
// false when cheerio throws on it. A selector juice preserves (a
// pseudo-element, :hover) matches nothing, as in css-select.
func attributeMatchers(sel string, o *options) ([]cascadia.Matcher, bool) {
	arms := selector.Split([]byte(sel))
	if len(arms) == 0 {
		return nil, false
	}
	var matchers []cascadia.Matcher
	for _, arm := range arms {
		c := selector.Compile(arm, selector.Mode{JuiceSpecificity: o.juiceSpecificity, Juice9: o.juice9Selectors, JuiceScope: o.juiceScope})
		if c.Invalid {
			return nil, false
		}
		if c.Ignored {
			continue
		}
		if len(c.Variants) == 0 {
			return nil, false
		}
		for _, v := range c.Variants {
			m, err := cascadia.ParseForJuice(v.Text, o.budget)
			if err != nil {
				return nil, false
			}
			matchers = append(matchers, m)
		}
	}
	return matchers, true
}

// setAttribute sets a on n as cheerio's attr(name, value): in place when n
// has it, last otherwise.
func setAttribute(n *html.Node, a html.Attribute) {
	for i := range n.Attr {
		if n.Attr[i].Key == a.Key {
			n.Attr[i].Val = a.Val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: a.Key, Val: a.Val})
}
