package components

import (
	"math"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
)

// The gutter of mj-section (MJML 5.4): the space between its columns, which
// mjml-column writes as a padding on each column (desktop and Outlook: on
// the sides between columns; mobile, where columns stack: above and below),
// and as a narrower desktop width. A port of mjml-column's getDesktopWidth,
// getNormalizedGutterValue, getDesktopPaddingValues, getMobilePaddingValues,
// getDesktopGutterClassName and their helpers.

// hasColumnGutter reports whether the column lays out with a gutter: its
// section (or the section of its group) sets one, not to the empty string.
// A gutter with no leading number, NaN in MJML, counts as none: safe-mjml
// writes no NaN (Pierre's rule).
func (c *MJColumnComponent) hasColumnGutter() bool {
	if !c.hasCtx || c.ctx.gutter == "" {
		return false
	}
	_, ok := parseMJMLWidth(c.ctx.gutter)
	return ok
}

// desktopParsedWidth is mjml-column's getParsedWidth: the width attribute,
// or an equal share of the non-raw siblings, as widthParser reads it, with
// its unit as written.
func (c *MJColumnComponent) desktopParsedWidth() (float64, string) {
	if width, ok := c.parsedWidthAttribute(); ok {
		return width.size.Value(), width.unit
	}
	return 100 / float64(max(c.getNonRawSiblings(), 1)), "%"
}

// gutterSiblings returns the props of mjml-core's renderChildren the gutter
// reads: the number of children of the parent, raw ones, comments and
// unknown tags included, and the column's index among them.
func (c *MJColumnComponent) gutterSiblings() (sibling, index int) {
	return max(c.ctx.Siblings, 1), c.ctx.index
}

// normalizedGutter is getNormalizedGutterValue: the gutter in targetUnit,
// converted between pixels and percentages of the container width, as
// written for any other pair of units.
func (c *MJColumnComponent) normalizedGutter(targetUnit string) float64 {
	gutter, ok := parseMJMLWidth(c.ctx.gutter)
	if !ok {
		return 0
	}
	value := gutter.size.Value()
	containerWidth := c.BaseComponent.layoutWidth()
	switch {
	case gutter.unit == targetUnit:
		return value
	case targetUnit == "%" && gutter.unit == "px":
		if containerWidth == 0 {
			return 0 // MJML would divide by zero
		}
		return value / containerWidth * 100
	case targetUnit == "px" && gutter.unit == "%":
		return containerWidth * value / 100
	}
	return value
}

// desktopWidth is getDesktopWidth: the column's width less its share of
// the gutters; in pixels, whole, the pixels left over going to the first
// columns.
func (c *MJColumnComponent) desktopWidth() (float64, string) {
	width, unit := c.desktopParsedWidth()
	if !c.hasColumnGutter() {
		if unit == "px" {
			return jsMathRound(width), unit
		}
		return width, unit
	}
	sibling, index := c.gutterSiblings()
	reduction := c.normalizedGutter(unit) * float64(sibling-1) / float64(sibling)
	reduced := math.Max(0, jsToFixed6(width-reduction))
	if unit == "px" {
		floor := math.Floor(reduced)
		extra := math.Max(0, math.Min(float64(sibling), jsMathRound(float64(sibling)*(reduced-floor))))
		if float64(index) < extra {
			floor++
		}
		return floor, unit
	}
	return reduced, unit
}

// gutterSides is a padding, top, right, bottom and left.
type gutterSides [4]float64

// desktopGutterPadding is getDesktopPaddingValues: half the gutter on each
// side between two columns (in pixels, the larger half leading), none on
// the outer sides, mirrored in rtl, and none for a column alone.
func (c *MJColumnComponent) desktopGutterPadding(unit string) gutterSides {
	sibling, index := c.gutterSiblings()
	if sibling == 1 {
		return gutterSides{}
	}
	gutter := c.normalizedGutter(unit)
	leading, trailing := gutter/2, gutter/2
	if unit == "px" {
		gutter = jsMathRound(gutter)
		leading, trailing = math.Ceil(gutter/2), math.Floor(gutter/2)
	}
	first, last := index == 0, index+1 == sibling
	edge := func(isEdge bool, half float64) float64 {
		if isEdge {
			return 0
		}
		return half
	}
	if c.ctx.direction == constants.DirectionRTL {
		return gutterSides{0, edge(first, trailing), 0, edge(last, leading)}
	}
	return gutterSides{0, edge(last, leading), 0, edge(first, trailing)}
}

// mobileGutterPadding is getMobilePaddingValues: half the gutter, as a
// percentage, above and below each column but the outer ones, where the
// columns stack.
func (c *MJColumnComponent) mobileGutterPadding() gutterSides {
	sibling, index := c.gutterSiblings()
	half := c.normalizedGutter("%") / 2
	first, last := index == 0, index+1 == sibling
	sides := gutterSides{half, 0, half, 0}
	if first {
		sides[0] = 0
	}
	if last {
		sides[2] = 0
	}
	return sides
}

// formatGutterPadding is formatPadding: each side rounded in pixels, cut to
// 6 decimals in any other unit.
func formatGutterPadding(sides gutterSides, unit string) string {
	parts := make([]string, len(sides))
	for i, v := range sides {
		if unit == "px" {
			parts[i] = jsNumber(jsMathRound(v)) + "px"
		} else {
			parts[i] = jsNumber(jsToFixed6(v)) + unit
		}
	}
	return strings.Join(parts, " ")
}

// desktopGutterCSS is getDesktopPadding, in the column's own unit.
func (c *MJColumnComponent) desktopGutterCSS() string {
	_, unit := c.desktopParsedWidth()
	return formatGutterPadding(c.desktopGutterPadding(unit), unit)
}

// mobileGutterCSS is getMobileGutterStyles' padding: the desktop one in a
// group, whose columns do not stack, the mobile one elsewhere.
func (c *MJColumnComponent) mobileGutterCSS() string {
	if c.Context().InsideGroup {
		return c.desktopGutterCSS()
	}
	return formatGutterPadding(c.mobileGutterPadding(), "%")
}

// outlookGutterCSS is getOutlookGutterStyles' padding, in pixels.
func (c *MJColumnComponent) outlookGutterCSS() string {
	return formatGutterPadding(c.desktopGutterPadding("px"), "px")
}

// gutterClassName is getDesktopGutterClassName:
// mj-column-gutter-{siblings}-{position}-{unit}-{gutter}[-rtl].
func (c *MJColumnComponent) gutterClassName() string {
	_, unit := c.desktopParsedWidth()
	gutter := c.normalizedGutter(unit)
	unitToken := unit
	if unit == "%" {
		unitToken = "per"
	}
	if unit == "px" {
		gutter = jsMathRound(gutter)
	}
	token := strings.Replace(jsNumber(jsToFixed6(gutter)), ".", "-", 1)
	sibling, index := c.gutterSiblings()
	name := "mj-column-gutter-" + strconv.Itoa(sibling) + "-" + strconv.Itoa(index+1) + "-" + unitToken + "-" + token
	if c.ctx.direction == constants.DirectionRTL {
		name += "-rtl"
	}
	return name
}

// GutterMediaQuery returns the class and the media-query rule of the
// column's gutter padding on desktop, as getColumnClass adds it, and false
// for a column without gutter or in a group, whose columns carry their
// padding inline.
func (c *MJColumnComponent) GutterMediaQuery() (string, string, bool) {
	if !c.hasColumnGutter() || c.Context().InsideGroup {
		return "", "", false
	}
	return c.gutterClassName(), "{ padding: " + c.desktopGutterCSS() + " !important; }", true
}
