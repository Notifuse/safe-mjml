package globals

import (
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// GlobalAttributes stores global attribute definitions from mj-attributes
type GlobalAttributes struct {
	// all stores mj-all global attributes that apply to all components
	all map[string]string
	// componentDefaults stores component-specific defaults (e.g., mj-text attributes)
	componentDefaults map[string]map[string]string
	// classDefaults stores mj-class definitions
	classDefaults map[string]map[string]string
	// classTagDefaults stores the tags written inside each mj-class, by class
	// then tag: the attributes they give the descendants of an element with
	// that class (classesDefault in mjml-core).
	classTagDefaults map[string]map[string]map[string]string
}

// NewGlobalAttributes creates a new global attributes store
func NewGlobalAttributes() *GlobalAttributes {
	return &GlobalAttributes{
		all:               make(map[string]string),
		componentDefaults: make(map[string]map[string]string),
		classDefaults:     make(map[string]map[string]string),
		classTagDefaults:  make(map[string]map[string]map[string]string),
	}
}

// ProcessAttributesFromHead processes mj-attributes from the head component
func (ga *GlobalAttributes) ProcessAttributesFromHead(headNode *parser.MJMLNode) {
	if headNode == nil {
		return
	}

	// Find mj-attributes elements
	for _, child := range headNode.Children {
		if child.XMLName.Local == "mj-attributes" {
			ga.processAttributesElement(child)
		}
	}
}

// processAttributesElement processes a single mj-attributes element
func (ga *GlobalAttributes) processAttributesElement(attributesNode *parser.MJMLNode) {
	for _, child := range attributesNode.Children {
		tagName := child.XMLName.Local

		switch tagName {
		case "mj-all":
			// Process mj-all - applies to all components
			for _, attr := range child.Attrs {
				ga.all[attr.Name.Local] = attr.Value
			}
		case "mj-class":
			// Process mj-class definitions. A class named "" is one, as
			// mjml-head-attributes adds it: mjml-core gives it to the
			// elements whose mj-class, split on a space, has an empty name
			// (no mj-class, an empty one, "a  b").
			className, named := "", false
			for _, attr := range child.Attrs {
				if attr.Name.Local == "name" {
					className, named = attr.Value, true
					break
				}
			}
			if !named {
				continue
			}
			if ga.classDefaults[className] == nil {
				ga.classDefaults[className] = make(map[string]string)
			}
			for _, attr := range child.Attrs {
				if attr.Name.Local == "name" {
					continue
				}
				ga.classDefaults[className][attr.Name.Local] = attr.Value
			}
			// The tags inside the class, as mjml-head-attributes adds them to
			// classesDefault: a later tag replaces an earlier one of the same
			// name, within the class and across its definitions.
			for _, tag := range child.Children {
				if ga.classTagDefaults[className] == nil {
					ga.classTagDefaults[className] = make(map[string]map[string]string)
				}
				attrs := make(map[string]string, len(tag.Attrs))
				for _, attr := range tag.Attrs {
					attrs[attr.Name.Local] = attr.Value
				}
				ga.classTagDefaults[className][tag.XMLName.Local] = attrs
			}
		default:
			// Process component-specific defaults (e.g., mj-text)
			if ga.componentDefaults[tagName] == nil {
				ga.componentDefaults[tagName] = make(map[string]string)
			}
			for _, attr := range child.Attrs {
				ga.componentDefaults[tagName][attr.Name.Local] = attr.Value
			}
		}
	}
}

// TagAttributes returns the attributes mj-attributes sets for one tag
// (<mj-attributes><mj-text color="..."/></mj-attributes>), empty ones
// included. The map must not be modified.
func (ga *GlobalAttributes) TagAttributes(componentName string) map[string]string {
	return ga.componentDefaults[componentName]
}

// LookupAllAttribute returns the value mj-all sets for an attribute, and
// whether it sets it, even to the empty string.
func (ga *GlobalAttributes) LookupAllAttribute(attrName string) (string, bool) {
	value, exists := ga.all[attrName]
	return value, exists
}

// GetClassAttributes returns all attributes for a given mj-class
func (ga *GlobalAttributes) GetClassAttributes(className string) map[string]string {
	if attrs, exists := ga.classDefaults[className]; exists {
		return attrs
	}
	return nil
}

// ClassTagAttributes returns the attributes the tag tagName written inside
// the mj-class className gives the descendants of an element with that class
// (classesDefault in mjml-core), or nil.
func (ga *GlobalAttributes) ClassTagAttributes(className, tagName string) map[string]string {
	return ga.classTagDefaults[className][tagName]
}

// HasClassTags reports whether the mj-class className has tags inside it,
// which give their attributes to the descendants of its elements.
func (ga *GlobalAttributes) HasClassTags(className string) bool {
	return len(ga.classTagDefaults[className]) > 0
}
