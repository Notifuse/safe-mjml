package mjml

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/juicer"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// htmlAttributeRule is an mj-selector of mj-html-attributes, with the node
// that first gave its path, for a warning.
type htmlAttributeRule struct {
	juicer.AttributeRule
	node *parser.MJMLNode
}

// htmlAttributeRules returns the rules of the head's mj-html-attributes, as
// mjml-head-html-attributes registers them in mjml-core's htmlAttributes:
// one per path, in the order paths first appear (a later mj-selector with the
// same path adds to it, its attributes replacing those of the same name in
// place), each with the attributes of its mj-html-attribute children that
// have a name, in order, the value being the element's text. A selector
// without path selects "undefined", as in MJML.
func (c *rootComponent) htmlAttributeRules() []htmlAttributeRule {
	if c.Head == nil {
		return nil
	}
	var rules []htmlAttributeRule
	byPath := map[string]int{}
	for _, child := range c.Head.GetChildren() {
		block, ok := child.(*components.MJHtmlAttributesComponent)
		if !ok {
			continue
		}
		for _, sel := range block.Node.Children {
			if sel.GetTagName() != "mj-selector" {
				continue
			}
			path, ok := sel.LookupAttribute("path")
			if !ok {
				path = "undefined"
			}
			i, seen := byPath[path]
			if !seen {
				i = len(rules)
				byPath[path] = i
				rules = append(rules, htmlAttributeRule{AttributeRule: juicer.AttributeRule{Selector: path}, node: sel})
			}
			for _, attr := range sel.Children {
				name := attr.GetAttribute("name")
				if attr.GetTagName() != "mj-html-attribute" || name == "" {
					continue
				}
				rules[i].Attributes = setRuleAttribute(rules[i].Attributes, html.Attribute{Key: name, Val: strings.TrimSpace(attr.Text)})
			}
		}
	}
	return rules
}

// setRuleAttribute sets a in attrs as an object spread does: in place when
// attrs has its name, last otherwise.
func setRuleAttribute(attrs []html.Attribute, a html.Attribute) []html.Attribute {
	for i := range attrs {
		if attrs[i].Key == a.Key {
			attrs[i].Val = a.Val
			return attrs
		}
	}
	return append(attrs, a)
}

// applyHTMLAttributes sets the attributes of rules on the elements of body,
// the rendered mj-body, as mjml-core does before it writes the skeleton:
// with cheerio in XML mode, which writes the body back, an element without
// children closing itself (<div/>). A selector cheerio cannot read sets
// nothing, with a warning; MJML fails on it.
func applyHTMLAttributes(body string, rules []htmlAttributeRule, opts *options.RenderOpts) (string, error) {
	attributeRules := make([]juicer.AttributeRule, len(rules))
	for i, r := range rules {
		attributeRules[i] = r.AttributeRule
	}
	out, invalid, err := juicer.SetAttributes(body, attributeRules,
		// Template tags kept intact, as inline mj-style keeps them.
		juicer.CodeBlocks(juicer.DefaultCodeBlocks()),
		juicer.MaxInputSize(0),
		juicer.MaxDepth(juiceMaxDepth),
		juicer.MaxMatchSteps(juiceMaxMatchSteps),
	)
	if err != nil {
		return "", fmt.Errorf("mj-html-attributes: %w", err)
	}
	for _, sel := range invalid {
		for _, r := range rules {
			if r.Selector == sel {
				opts.Warn(options.NewWarning(options.WarningInvalidAttribute, r.node, fmt.Sprintf("mj-selector path %q is not a valid selector: it sets no attribute (MJML fails on it)", sel)))
				break
			}
		}
	}
	return out, nil
}
