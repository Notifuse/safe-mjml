package components

import (
	"math"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/styles"
)

// ChildContext is what a container gives each of its children, as MJML's
// getChildContext and the sibling counts of renderChildren do. The tree
// builder computes it with ChildContextOf and sets it once on each child,
// before building the child's own children, so every pass of a render, from
// the head styles to Render, sees the same widths.
type ChildContext struct {
	// ContainerWidth is the width in pixels the child lays out in, never
	// negative. Like the containerWidth string of MJML's context, it can carry
	// a fraction (402.67 for 67% of 601px): the readers that use parseFloat in
	// MJML read it whole (layoutWidth), the others truncate it like parseInt
	// (GetEffectiveWidth).
	ContainerWidth float64
	// containerWidthText is the container width as the body's width
	// attribute writes it (600 without a unit), for the blocks of mj-body,
	// which MJML writes from that string; "" elsewhere, where it is
	// jsPixels(ContainerWidth).
	containerWidthText string
	// Siblings and RawSiblings count the children of the parent, the child
	// itself included, and those of them that are raw elements.
	Siblings    int
	RawSiblings int
	// InsideGroup is set for the children of an mj-group.
	InsideGroup bool
	// InSection is set for the children of an mj-section, whose Outlook row
	// opens a cell for each of them.
	InSection bool
	// PassedAttributes are the attributes the parent passes down to its
	// children, as the attributes of renderChildren in mjml-core: they come
	// after the child's own (element, mj-class, mj-attributes for its tag)
	// and before mj-all and the defaults. They are not passed further down.
	PassedAttributes map[string]string
	// classDefaults holds the tags inside the classes of the nearest ancestor
	// that sets mj-class, even to "" (nil then, which cuts the chain): they
	// give the child their attributes (classesDefault in mjml-core). It is
	// built once for that ancestor and goes down the whole tree.
	classDefaults *classTagDefaults
	// index is the child's position among its parent's children, raw
	// children and comments included (props.index in mjml-core); AddChild
	// sets it.
	index int
	// gap is the gap of the parent mj-wrapper (context.gap in MJML): the
	// space above each section but the first.
	gap string
}

// childContextProvider is a container that gives its children a context of
// its own, as MJML containers override getChildContext; inherited is the
// context they get by default.
type childContextProvider interface {
	childContext(inherited ChildContext) ChildContext
}

// ChildContextOf returns the context parent gives each of its children, who
// are siblings in all, rawSiblings of them raw elements. By default, as in
// MJML, it is the parent's own context, with the parent's width as the
// container width; the flags that hold only for direct children
// (InsideGroup, InSection) are not passed further down. A container with a
// childContext method adjusts it.
func ChildContextOf(parent Component, siblings, rawSiblings int) ChildContext {
	ctx := parent.Context()
	if p, ok := parent.(interface {
		childClassDefaults() (*classTagDefaults, bool)
	}); ok {
		if defaults, set := p.childClassDefaults(); set {
			ctx.classDefaults = defaults
		}
	}
	ctx.InsideGroup = false
	ctx.InSection = false
	ctx.PassedAttributes = nil
	ctx.Siblings = siblings
	ctx.RawSiblings = rawSiblings
	ctx.ContainerWidth = parent.layoutWidth()
	ctx.containerWidthText = ""
	if p, ok := parent.(childContextProvider); ok {
		ctx = p.childContext(ctx)
	}
	return ctx
}

// childContext gives the columns and groups of a section its box width.
func (c *MJSectionComponent) childContext(ctx ChildContext) ChildContext {
	ctx.ContainerWidth = float64(c.childContainerWidth())
	ctx.InSection = true
	// As mjml-section's getChildContext: its own gap, which a section only
	// gets from mj-all.
	ctx.gap = c.resolveAttribute(c, constants.MJMLGap)
	return ctx
}

// childContext gives the sections and heroes of a wrapper its box width.
func (c *MJWrapperComponent) childContext(ctx ChildContext) ChildContext {
	ctx.ContainerWidth = float64(c.childContainerWidth())
	ctx.gap = c.resolveAttribute(c, constants.MJMLGap)
	return ctx
}

// childContext gives the content of a column its content width.
func (c *MJColumnComponent) childContext(ctx ChildContext) ChildContext {
	ctx.ContainerWidth = c.contentWidth()
	return ctx
}

// childContext gives the columns of a group the group's width minus its
// left and right padding, as mjml-group's getChildContext computes it.
func (c *MJGroupComponent) childContext(ctx ChildContext) ChildContext {
	ctx.ContainerWidth = max(c.childBaseWidth(c)-
		float64(c.shorthandAttrValue(c, constants.MJMLPadding, sideLeft))-
		float64(c.shorthandAttrValue(c, constants.MJMLPadding, sideRight)), 0)
	ctx.InsideGroup = true
	return ctx
}

// childContext gives the content of a hero its width minus its padding.
func (c *MJHeroComponent) childContext(ctx ChildContext) ChildContext {
	ctx.ContainerWidth = c.childContainerWidth()
	return ctx
}

// childBaseWidth returns the width a column or group lays its children out
// in before its own box sides, as mjml-column's and mjml-group's
// getChildContext take it: its width attribute in its container, or, without
// one, an equal share of the container read as pixels, which widthParser
// truncates like parseInt.
func (bc *BaseComponent) childBaseWidth(comp interface {
	Component
	exactWidth() float64
}) float64 {
	if bc.resolveAttribute(comp, constants.MJMLWidth) != "" {
		return comp.exactWidth()
	}
	return float64(lengthFromFloat(bc.layoutWidth() / float64(max(bc.getNonRawSiblings(), 1))))
}

// widthIn returns a width parsed as MJML's widthParser reads it, in a
// container of containerWidth pixels: a percentage of the container, read
// with parseFloat, or pixels, already truncated.
func widthIn(size styles.Size, containerWidth float64) float64 {
	if size.IsPercent() {
		return containerWidth * size.Value() / 100
	}
	return size.Value()
}

// containerWidthCSS returns the width the component lays out in as MJML's
// containerWidth string writes it: the body's width as written for a block
// of mj-body, else the parent's box in pixels, with its fraction (600.5px).
func (bc *BaseComponent) containerWidthCSS() string {
	if bc.hasCtx && bc.ctx.containerWidthText != "" {
		return bc.ctx.containerWidthText
	}
	return jsPixels(bc.layoutWidth())
}

// jsPixels writes a length in pixels as JavaScript prints `${x}px`
// (199.98px).
func jsPixels(v float64) string {
	return jsNumber(v) + "px"
}

// jsNumber writes v as JavaScript's String(v) does: the shortest decimal
// that reads back as v, 0 for -0, in exponent notation (1e+21, 1.25e-7)
// from 1e21 and below 1e-6, and Infinity or -Infinity.
func jsNumber(v float64) string {
	switch {
	case v == 0:
		return "0"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	case math.Abs(v) >= 1e21 || math.Abs(v) < 1e-6:
		// Go writes the exponent with at least two digits (1e-07).
		mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(v, 'e', -1, 64), "e")
		expSign, expDigits := exponent[:1], strings.TrimLeft(exponent[1:], "0")
		return mantissa + "e" + expSign + expDigits
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// passedAttributes returns the attributes of names comp resolves, to pass
// down to its children: those it sets or has a default for, as MJML passes
// getAttribute(name); one it does not have is not passed.
func (bc *BaseComponent) passedAttributes(comp Component, names map[string]struct{}) map[string]string {
	passed := make(map[string]string, len(names))
	for name := range names {
		if bc.hasAttribute(comp, name) {
			passed[name] = bc.resolveAttribute(comp, name)
		}
	}
	return passed
}
