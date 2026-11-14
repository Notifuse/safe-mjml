package mjml

import (
	"fmt"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// mjmlDependency lists the children MJML allows in one tag. anyChild marks
// mj-attributes, whose entry in MJML is a regular expression matching every
// tag.
type mjmlDependency struct {
	parent   string
	children []string
	anyChild bool
}

// mjmlDependencies is mjml-preset-core/lib/dependencies.js of MJML 4.18, in
// its original order (the "only inside" list of a misplaced-tag warning
// follows it).
var mjmlDependencies = []mjmlDependency{
	{"mjml", []string{"mj-body", "mj-head", "mj-raw"}, false},
	{"mj-accordion", []string{"mj-accordion-element", "mj-raw"}, false},
	{"mj-accordion-element", []string{"mj-accordion-title", "mj-accordion-text", "mj-raw"}, false},
	{"mj-accordion-title", nil, false},
	{"mj-accordion-text", nil, false},
	{"mj-attributes", nil, true},
	{"mj-body", []string{"mj-raw", "mj-section", "mj-wrapper", "mj-hero"}, false},
	{"mj-button", nil, false},
	{"mj-carousel", []string{"mj-carousel-image"}, false},
	{"mj-carousel-image", nil, false},
	{"mj-column", []string{"mj-accordion", "mj-button", "mj-carousel", "mj-divider", "mj-image", "mj-raw", "mj-social", "mj-spacer", "mj-table", "mj-text", "mj-navbar"}, false},
	{"mj-html-attribute", nil, false},
	{"mj-html-attributes", []string{"mj-selector"}, false},
	{"mj-divider", nil, false},
	{"mj-group", []string{"mj-column", "mj-raw"}, false},
	{"mj-head", []string{"mj-attributes", "mj-breakpoint", "mj-html-attributes", "mj-font", "mj-preview", "mj-style", "mj-title", "mj-raw"}, false},
	{"mj-hero", []string{"mj-accordion", "mj-button", "mj-carousel", "mj-divider", "mj-image", "mj-social", "mj-spacer", "mj-table", "mj-text", "mj-navbar", "mj-raw"}, false},
	{"mj-image", nil, false},
	{"mj-navbar", []string{"mj-navbar-link", "mj-raw"}, false},
	{"mj-raw", nil, false},
	{"mj-section", []string{"mj-column", "mj-group", "mj-raw"}, false},
	{"mj-selector", []string{"mj-html-attribute"}, false},
	{"mj-social", []string{"mj-social-element", "mj-raw"}, false},
	{"mj-social-element", nil, false},
	{"mj-spacer", nil, false},
	{"mj-table", nil, false},
	{"mj-text", nil, false},
	{"mj-wrapper", []string{"mj-hero", "mj-raw", "mj-section"}, false},
}

// unsupportedTags are MJML tags safe-mjml does not implement: they are left out
// with a warning saying so, rather than reported as unknown.
var unsupportedTags = map[string]bool{
	"mj-html-attributes": true,
	"mj-html-attribute":  true,
	"mj-selector":        true,
	"mj-include":         true,
}

// allowedChildren returns the children MJML allows in parent, and whether
// MJML lists parent at all.
func allowedChildren(parent string) ([]string, bool) {
	for _, dep := range mjmlDependencies {
		if dep.parent == parent {
			return dep.children, true
		}
	}
	return nil, false
}

// isMJMLTag reports whether MJML 4.18 knows tag: it appears in its
// dependency table, as a parent or as an allowed child.
func isMJMLTag(tag string) bool {
	for _, dep := range mjmlDependencies {
		if dep.parent == tag {
			return true
		}
		for _, c := range dep.children {
			if c == tag {
				return true
			}
		}
	}
	return false
}

// allowedParents returns, in dependencies.js order, the tags that allow child.
func allowedParents(child string) []string {
	var parents []string
	for _, dep := range mjmlDependencies {
		if dep.anyChild {
			parents = append(parents, dep.parent)
			continue
		}
		for _, c := range dep.children {
			if c == child {
				parents = append(parents, dep.parent)
				break
			}
		}
	}
	return parents
}

// warnUnknownTag reports a child that is not a component safe-mjml renders.
func warnUnknownTag(opts *options.RenderOpts, node *parser.MJMLNode) {
	tag := node.GetTagName()
	message := fmt.Sprintf("Element %s doesn't exist or is not registered", tag)
	kind := options.WarningUnknownTag
	if unsupportedTags[tag] {
		message = fmt.Sprintf("Element %s is not supported by safe-mjml and is ignored", tag)
		kind = options.WarningUnsupportedTag
	}
	opts.Warn(options.NewWarning(kind, node, message))
}

// warnIfMisplaced reports a known component placed where MJML does not allow
// it, with mjml-validator's validChildren message. The component is rendered
// all the same, as MJML does with soft validation.
func warnIfMisplaced(opts *options.RenderOpts, parent string, node *parser.MJMLNode) {
	allowed, listed := allowedChildren(parent)
	if !listed {
		return
	}
	child := node.GetTagName()
	for _, c := range allowed {
		if c == child {
			return
		}
	}
	opts.Warn(options.NewWarning(options.WarningMisplacedTag, node, fmt.Sprintf("%s cannot be used inside %s, only inside: %s", child, parent, strings.Join(allowedParents(child), ", "))))
}

// componentLessTags may appear inside mj-attributes without being components;
// MJML does not validate their attributes either.
var componentLessTags = map[string]bool{"mj-all": true, "mj-class": true}

// validateAttributeDefinitions checks the children of an mj-attributes
// element as mjml-validator does: an unknown tag is reported, and the
// attributes defined for a component tag (<mj-text foo="x"/>) must be ones
// that tag accepts. mj-all and mj-class are not checked.
func validateAttributeDefinitions(opts *options.RenderOpts, attributesNode *parser.MJMLNode) {
	for _, def := range attributesNode.Children {
		tag := def.GetTagName()
		switch {
		case componentLessTags[tag]:
		case isMJMLTag(tag) && !unsupportedTags[tag]:
			components.ValidateAttributes(def, opts)
		default:
			warnUnknownTag(opts, def)
		}
	}
}
