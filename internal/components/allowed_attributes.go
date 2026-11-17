package components

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	_ "embed"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

//go:embed allowed-css-attributes.json
var allowedCSSAttributesJSON []byte

// allowedAttribute is an attribute a component accepts, with its type.
type allowedAttribute struct {
	spec string
	typ  *attributeType
}

// allowedAttributes returns, per tag, the attributes MJML 5.4.1 accepts and
// their checkers, built once on first use: a render then reads them without
// a lock, and a program that never renders pays nothing.
var allowedAttributes = sync.OnceValue(func() map[string]map[string]allowedAttribute {
	var specs map[string]map[string]string
	if err := json.Unmarshal(allowedCSSAttributesJSON, &specs); err != nil {
		panic(fmt.Errorf("failed to parse allowed CSS attributes: %w", err))
	}
	types := map[string]*attributeType{}
	out := make(map[string]map[string]allowedAttribute, len(specs))
	for tag, attrs := range specs {
		m := make(map[string]allowedAttribute, len(attrs))
		for name, spec := range attrs {
			t, ok := types[spec]
			if !ok {
				t = newAttributeType(spec)
				types[spec] = t
			}
			m[name] = allowedAttribute{spec: spec, typ: t}
		}
		out[tag] = m
	}
	return out
})

// isGloballyAllowedAttribute reports whether every component accepts the
// attribute: mj-class and css-class, mjml-validator's whitelist. class,
// data-* and aria-* are illegal on an mj-* tag, as in MJML.
func isGloballyAllowedAttribute(attrName string) bool {
	return attrName == "mj-class" || attrName == "css-class"
}

// ValidateAttributes reports, as invalid-attribute warnings, the attributes
// of node that its tag does not accept. Components check their own when they
// are created; this is for elements that are not built as components, such
// as the per-tag definitions inside mj-attributes.
func ValidateAttributes(node *parser.MJMLNode, opts *options.RenderOpts) {
	validateComponentAttributes(node, opts)
}

func validateComponentAttributes(node *parser.MJMLNode, opts *options.RenderOpts) {
	if node == nil || opts == nil || opts.WarningHandler == nil {
		return
	}

	tagName := node.GetTagName()
	allowed, ok := allowedAttributes()[tagName]
	if !ok {
		return
	}

	var illegal []string
	for _, attr := range node.Attrs {
		name := attr.Name.Local
		if isGloballyAllowedAttribute(name) {
			continue
		}
		if _, exists := allowed[name]; exists {
			continue
		}
		illegal = append(illegal, name)
	}
	// One warning for all of them, as mjml-validator's validAttributes.
	switch len(illegal) {
	case 0:
	case 1:
		opts.Warn(options.NewWarning(options.WarningInvalidAttribute, node, "Attribute "+illegal[0]+" is illegal"))
	default:
		opts.Warn(options.NewWarning(options.WarningInvalidAttribute, node, "Attributes "+strings.Join(illegal, ", ")+" are illegal"))
	}
	// Then the values of the allowed ones, as mjml-validator's validTypes
	// rule, which runs after validAttributes.
	for _, attr := range node.Attrs {
		a, ok := allowed[attr.Name.Local]
		if !ok {
			continue
		}
		if message := a.typ.check(attr.Value); message != "" {
			opts.Warn(options.NewWarning(options.WarningInvalidAttributeValue, node, "Attribute "+attr.Name.Local+" "+message))
		}
	}
}
