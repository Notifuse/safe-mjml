package components

import (
	"io"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/globals"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
	"github.com/Notifuse/safe-mjml/internal/styles"
)

// Common width strings to avoid fmt.Sprintf allocations
var (
	width600px = "600px"
	width100px = "100px"
	width200px = "200px"
	width300px = "300px"
	width400px = "400px"
	width500px = "500px"
	width150px = "150px"
	width50px  = "50px"
)

// pixelWidthStringBufSize is the pre-allocated buffer size for pixel width strings
// Calculation: max 4-5 digits + "px" suffix = ~7-8 chars
const pixelWidthStringBufSize = 8

// Component represents a renderable MJML component
type Component interface {
	Render(w io.StringWriter) error
	GetTagName() string
	GetDefaultAttribute(name string) string
	// SetContext gives the component the context its parent gives its
	// children (see ChildContextOf); the tree builder sets it once, before
	// building the component's own children. A tree assembled by hand with
	// AddChild must set it too: without it, a component lays out in the
	// default body width as an only child.
	SetContext(ctx ChildContext)
	// Context returns the context set by SetContext, or the zero context.
	Context() ChildContext
	// GetEffectiveWidth returns the component's width in whole pixels, as the
	// MJML readers that use parseInt read it.
	GetEffectiveWidth() int
	// layoutWidth returns the width the component gives its children unless
	// it has a childContext of its own, with its fraction, as MJML passes the
	// containerWidth string down.
	layoutWidth() float64
	IsRawElement() bool
	// GetChildren returns the child components, in document order.
	GetChildren() []Component
	// AddChild appends a child component; the tree builder uses it. Children
	// are added only through it, so the containers that keep track of some of
	// theirs (an accordion's elements, an element's title and text) stay in
	// step. It returns an error for a child the component cannot take.
	AddChild(child Component) error
}

// BaseComponent provides common functionality for all components
type BaseComponent struct {
	Node     *parser.MJMLNode
	children []Component // set through AddChild only
	// own holds the attributes MJML merges into the component's own, once, at
	// construction: mj-attributes for its tag, then its mj-class, then the
	// element, each layer over the one before (applyAttributes in mjml-core).
	// An attribute a layer sets to "" is set: it hides the layers below and
	// the default.
	own map[string]string
	// ctx is what the parent gives the component, once hasCtx; a component
	// without it (the root, or one built alone) takes the default body width
	// and counts as an only child.
	ctx        ChildContext
	hasCtx     bool
	RenderOpts *options.RenderOpts // Rendering options
}

// NewBaseComponent creates a new base component
func NewBaseComponent(node *parser.MJMLNode, opts *options.RenderOpts) *BaseComponent {
	if opts == nil {
		opts = &options.RenderOpts{}
	}

	// mj-attributes for the tag, then mj-class, then the element.
	var tagAttrs map[string]string
	ga := opts.GlobalAttributes
	if ga != nil {
		tagAttrs = ga.TagAttributes(node.GetTagName())
	}
	own := make(map[string]string, len(node.Attrs)+len(tagAttrs))
	if ga != nil {
		for name, value := range tagAttrs {
			own[name] = normalizeAttributeValue(name, value)
		}
		// As applyAttributes in mjml-core, split on a single space
		// (split(' ')), with no mj-class read as "": an empty name is a
		// class, the one an mj-class named "" defines (usually none, and
		// then an element without mj-class has nothing to merge). The
		// attributes of a later class win, except that two non-empty
		// css-class add up; an empty one replaces the css-class before it,
		// and is replaced by the next.
		if classAttr := node.GetAttribute(constants.MJMLMJClass); classAttr != "" || ga.GetClassAttributes("") != nil {
			classNames := strings.Split(classAttr, " ")
			// The css-class of every occurrence, joined once.
			var cssClasses []string
			hasCSSClass := false
			for _, className := range classNames {
				value, ok := ga.GetClassAttributes(className)[constants.MJMLCSSClass]
				if !ok {
					continue
				}
				hasCSSClass = true
				if value == "" {
					cssClasses = cssClasses[:0]
				} else {
					cssClasses = append(cssClasses, value)
				}
			}
			// The other attributes: only the last occurrence of a class
			// decides them, so each distinct class is merged once.
			for _, className := range lastOccurrences(classNames) {
				for name, value := range ga.GetClassAttributes(className) {
					if name != constants.MJMLCSSClass {
						own[name] = normalizeAttributeValue(name, value)
					}
				}
			}
			if hasCSSClass {
				own[constants.MJMLCSSClass] = strings.Join(cssClasses, " ")
			}
		}
	}
	for _, attr := range node.Attrs {
		if name := attr.Name.Local; name != constants.MJMLMJClass {
			own[name] = normalizeAttributeValue(name, attr.Value)
		}
	}

	bc := &BaseComponent{
		Node:       node,
		own:        own,
		children:   make([]Component, 0, len(node.Children)),
		RenderOpts: opts,
	}

	validateComponentAttributes(node, opts)

	return bc
}

func normalizeAttributeValue(name, value string) string {
	if value == "" {
		return value
	}

	if strings.Contains(strings.ToLower(name), "color") {
		return styles.NormalizeColor(value)
	}

	return value
}

// GetChildren returns the child components, in document order.
func (bc *BaseComponent) GetChildren() []Component {
	return bc.children
}

// AddChild appends a child component.
func (bc *BaseComponent) AddChild(child Component) error {
	if indexed, ok := child.(interface{ setIndex(int) }); ok {
		indexed.setIndex(len(bc.children))
	}
	bc.children = append(bc.children, child)
	return nil
}

// setIndex records the component's position among its parent's children
// (ChildContext.index).
func (bc *BaseComponent) setIndex(index int) {
	bc.ctx.index = index
}

// IsRawElement returns whether this component should be treated as a raw element.
// Base components are not raw by default.
func (bc *BaseComponent) IsRawElement() bool {
	return false
}

// getAttribute returns the attribute as MJML's getAttribute gives it,
// without the component default: from the element, mj-class, mj-attributes,
// its parent or mj-all (see lookupAttribute); nil when none sets it, or sets
// it to the empty string.
func (bc *BaseComponent) getAttribute(name string) *string {
	if value, ok := bc.lookupAttribute(name); ok && value != "" {
		return &value
	}
	return nil
}

// resolveAttribute returns the attribute as MJML resolves it for the
// component: lookupAttribute, then the component default. An attribute set
// to the empty string stays empty, as in MJML.
func (bc *BaseComponent) resolveAttribute(comp Component, name string) string {
	if value, ok := bc.lookupAttribute(name); ok {
		return value
	}
	return normalizeAttributeValue(name, comp.GetDefaultAttribute(name))
}

// lookupAttribute returns an attribute from the layers mjml-core merges
// above the component defaults, each over the one after it: the component's
// own (the element, its mj-class, mj-attributes for its tag), the attributes
// its parent passes down (ChildContext.PassedAttributes), then mj-all; and
// whether one of them sets it, even to the empty string.
func (bc *BaseComponent) lookupAttribute(name string) (string, bool) {
	if value, ok := bc.own[name]; ok {
		return value, true
	}
	if value, ok := bc.ctx.PassedAttributes[name]; ok {
		return value, true
	}
	if bc.RenderOpts != nil && bc.RenderOpts.GlobalAttributes != nil {
		if value, ok := bc.RenderOpts.GlobalAttributes.LookupAllAttribute(name); ok {
			return normalizeAttributeValue(name, value), true
		}
	}
	return "", false
}

// hasAttribute reports whether an attribute is set on the component, as
// MJML's getAttribute(name) != null tests it: by one of the layers of
// lookupAttribute, even to the empty string, or by a default.
func (bc *BaseComponent) hasAttribute(comp Component, name string) bool {
	if _, ok := bc.lookupAttribute(name); ok {
		return true
	}
	return comp.GetDefaultAttribute(name) != ""
}

// GetDefaultAttribute returns the default value for an attribute.
// Override this method in specific components to provide component-specific defaults.
func (bc *BaseComponent) GetDefaultAttribute(name string) string {
	return ""
}

// SetContext sets the context the parent gives this component. A negative
// container width, which MJML can compute (a padding wider than its box), is
// set as 0: the component is then no wider than its parent.
func (bc *BaseComponent) SetContext(ctx ChildContext) {
	ctx.ContainerWidth = max(ctx.ContainerWidth, 0)
	bc.ctx = ctx
	bc.hasCtx = true
	bc.applyClassTagDefaults()
}

// applyClassTagDefaults merges into the component's own attributes those
// that the tags inside the mj-class of its nearest classed ancestor give its
// tag, as applyAttributes in mjml-core: above mj-attributes for the tag and
// the component's own mj-class, below the element, a later class winning.
// The element's attributes are indexed once, so that the cost is linear in
// the defaults and the attributes.
func (bc *BaseComponent) applyClassTagDefaults() {
	defaults := bc.ctx.classDefaults.forTag(bc.Node.GetTagName())
	if len(defaults) == 0 {
		return
	}
	explicit := make(map[string]struct{}, len(bc.Node.Attrs))
	for _, attr := range bc.Node.Attrs {
		explicit[attr.Name.Local] = struct{}{}
	}
	for name, value := range defaults {
		if _, set := explicit[name]; !set {
			bc.own[name] = normalizeAttributeValue(name, value)
		}
	}
}

// childClassDefaults returns, when the element sets mj-class (even empty),
// the class tags it gives its descendants, and true. An element no parent
// gives a context (mj-body and mj-head, which the root builds) without one
// gives those of the class named "", as mjml-core starts from an empty
// parent mj-class at the root.
func (bc *BaseComponent) childClassDefaults() (*classTagDefaults, bool) {
	class, set := nodeAttribute(bc.Node, constants.MJMLMJClass)
	if !set && bc.hasCtx {
		return nil, false
	}
	return newClassTagDefaults(bc.RenderOpts.GlobalAttributes, class), true
}

// classTagDefaults resolves, for the classes of one element, the attributes
// their inner tags give each tag; a tag's merge is computed once and shared
// by all the element's descendants.
type classTagDefaults struct {
	ga      *globals.GlobalAttributes
	classes []string
	byTag   map[string]map[string]string
}

// newClassTagDefaults reads an mj-class value as mjml-core does for
// classesDefault: split on a single space (a tab is part of a name), each
// class then looked up as a lodash path (get(classesDefault,
// `${class}.${tag}`)), so a name with ".", "[" or "]" finds nothing. It
// returns nil when no class can give anything.
func newClassTagDefaults(ga *globals.GlobalAttributes, mjClass string) *classTagDefaults {
	if ga == nil {
		return nil
	}
	var classes []string
	// Only the last occurrence of a class decides what it gives, so each
	// distinct class is merged once.
	for _, class := range lastOccurrences(strings.Split(mjClass, " ")) {
		if !strings.ContainsAny(class, ".[]") && ga.HasClassTags(class) {
			classes = append(classes, class)
		}
	}
	if len(classes) == 0 {
		return nil
	}
	return &classTagDefaults{ga: ga, classes: classes}
}

// forTag returns the attributes the classes give tag, a later class winning
// attribute by attribute; nil for a nil receiver.
func (d *classTagDefaults) forTag(tag string) map[string]string {
	if d == nil {
		return nil
	}
	if attrs, done := d.byTag[tag]; done {
		return attrs
	}
	var merged map[string]string
	for _, class := range d.classes {
		for name, value := range d.ga.ClassTagAttributes(class, tag) {
			if merged == nil {
				merged = make(map[string]string)
			}
			merged[name] = value
		}
	}
	if d.byTag == nil {
		d.byTag = make(map[string]map[string]string)
	}
	d.byTag[tag] = merged
	return merged
}

// nodeAttribute returns the attribute written on node, and whether it is
// written, even empty.
func nodeAttribute(node *parser.MJMLNode, name string) (string, bool) {
	for _, attr := range node.Attrs {
		if attr.Name.Local == name {
			return attr.Value, true
		}
	}
	return "", false
}

// Context returns the context the parent gave this component, or the zero
// context.
func (bc *BaseComponent) Context() ChildContext {
	return bc.ctx
}

// getNonRawSiblings returns the number of the component's siblings, itself
// included, that are not raw elements; 1 without a context.
func (bc *BaseComponent) getNonRawSiblings() int {
	if !bc.hasCtx {
		return 1
	}
	return bc.ctx.Siblings - bc.ctx.RawSiblings
}

// GetEffectiveWidth returns the container width if set, otherwise default body width
// AIDEV-NOTE: width-flow-calculation; used to calculate actual pixel width for rendering and child width calculation
func (bc *BaseComponent) GetEffectiveWidth() int {
	if bc.hasCtx {
		return lengthFromFloat(bc.ctx.ContainerWidth)
	}
	return GetDefaultBodyWidthPixels()
}

// layoutWidth returns the width the parent gives the component with its
// fraction, as the MJML readers that use parseFloat read it; GetEffectiveWidth
// truncates it like parseInt.
func (bc *BaseComponent) layoutWidth() float64 {
	if bc.hasCtx {
		return bc.ctx.ContainerWidth
	}
	return float64(GetDefaultBodyWidthPixels())
}

// getPixelWidthString returns pixel width string, using cached values for common widths to avoid allocations
func getPixelWidthString(widthPx int) string {
	switch widthPx {
	case 600:
		return width600px
	case 500:
		return width500px
	case 400:
		return width400px
	case 300:
		return width300px
	case 200:
		return width200px
	case 150:
		return width150px
	case 100:
		return width100px
	case 50:
		return width50px
	default:
		// Fallback using strconv for uncommon widths without fmt overhead
		var b strings.Builder
		b.Grow(pixelWidthStringBufSize) // Pre-allocate reasonable size for most width values
		b.WriteString(strconv.Itoa(widthPx))
		b.WriteString("px")
		return b.String()
	}
}

// Style Mixin Methods - Common styling patterns that components can use

// addDebugAttribute adds a debug attribute to an HTML tag for component traceability
// This helps identify which MJML component generated which HTML elements during testing
func (bc *BaseComponent) addDebugAttribute(tag *html.HTMLTag, componentType string) {
	// Only add debug attributes if enabled in render options
	if bc.RenderOpts != nil && bc.RenderOpts.DebugTags {
		debugAttr := "data-mj-debug-" + componentType
		tag.AddAttribute(debugAttr, "true")
	}
}

// CSS Class Helper Methods - Generic css-class attribute handling for all components

// getCSSClass returns the css-class attribute value
func (bc *BaseComponent) getCSSClass() string {
	value, _ := bc.lookupAttribute(constants.MJMLCSSClass)
	return value
}

// buildClassAttribute combines existing CSS classes with the css-class attribute
// Usage: component.buildClassAttribute("mj-column-per-100", "mj-outlook-group-fix")
func (bc *BaseComponent) buildClassAttribute(existingClasses ...string) string {
	// The css-class resolved from the element, mj-class and mj-attributes
	cssClass := bc.getCSSClass()

	// Count total classes
	total := 0
	for _, class := range existingClasses {
		if class != "" {
			total++
		}
	}
	if cssClass != "" {
		total++
	}

	if total == 0 {
		return ""
	}

	if total == 1 {
		for _, class := range existingClasses {
			if class != "" {
				return class
			}
		}
		return cssClass
	}

	// More than one class: build string efficiently
	// Pre-calculate buffer size
	totalLen := total - 1 // spaces between classes
	for _, class := range existingClasses {
		if class != "" {
			totalLen += len(class)
		}
	}
	if cssClass != "" {
		totalLen += len(cssClass)
	}

	var b strings.Builder
	b.Grow(totalLen)
	first := true
	for _, class := range existingClasses {
		if class == "" {
			continue
		}
		if !first {
			b.WriteByte(' ')
		} else {
			first = false
		}
		b.WriteString(class)
	}
	if cssClass != "" {
		if !first {
			b.WriteByte(' ')
		}
		b.WriteString(cssClass)
	}
	return b.String()
}

// setClassAttribute applies the computed class attribute to the provided tag.
func (bc *BaseComponent) setClassAttribute(tag *html.HTMLTag, existingClasses ...string) string {
	classAttr := bc.buildClassAttribute(existingClasses...)
	if classAttr != "" {
		tag.AddAttribute(constants.AttrClass, classAttr)
	}
	return classAttr
}

// lastOccurrences returns names without repetitions, each at the place of its
// last occurrence: merging their attributes in that order gives what merging
// every occurrence does, a later one winning, once per distinct name.
func lastOccurrences(names []string) []string {
	last := make(map[string]int, len(names))
	for i, name := range names {
		last[name] = i
	}
	if len(last) == len(names) {
		return names
	}
	out := make([]string, 0, len(last))
	for i, name := range names {
		if last[name] == i {
			out = append(out, name)
		}
	}
	return out
}

// suffixCSSClasses appends -suffix to every class of a css-class value, as
// mjml-core's suffixCssClasses does ("a b" gives "a-outlook b-outlook"); an
// empty value gives "".
func suffixCSSClasses(classes, suffix string) string {
	if classes == "" {
		return ""
	}
	parts := strings.Split(classes, " ")
	for i, class := range parts {
		parts[i] = class + "-" + suffix
	}
	return strings.Join(parts, " ")
}

// emptyAttributesDropped are the attributes MJML's minifier (html-minifier,
// removeEmptyAttributes) drops when empty; it keeps the others, rel="" or
// role="" for instance.
var emptyAttributesDropped = map[string]bool{"class": true, "id": true, "style": true, "title": true, "lang": true, "dir": true}

// addSetAttribute writes the HTML attribute name with its resolved value when
// a layer sets it, as mjml-core's htmlAttributes writes every non-nil value,
// except an empty one the minifier drops.
func (bc *BaseComponent) addSetAttribute(comp Component, tag *html.HTMLTag, name string) {
	if !bc.hasAttribute(comp, name) {
		return
	}
	value := bc.resolveAttribute(comp, name)
	if value == "" && emptyAttributesDropped[name] {
		return
	}
	tag.AddAttribute(name, value)
}
