package components

import (
	"io"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/fonts"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJButtonComponent represents mj-button
type MJButtonComponent struct {
	*BaseComponent
}

// NewMJButtonComponent creates a new mj-button component
func NewMJButtonComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJButtonComponent {
	return &MJButtonComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

// linkWidth returns the width of the button's link, as mjml-button's
// calculateAWidth: for a pixel width, the width less the left and right
// inner padding and borders; nothing for another unit (a percentage
// depends on the text) or a width that is not a number. A width parseInt
// reads as Infinity is saturated at ±maxLength, as other lengths are:
// safe-mjml writes no Infinity. The sum is in float64, so that saturated
// paddings cannot overflow a 32-bit int.
func (c *MJButtonComponent) linkWidth(width string) string {
	if width == "" || widthUnit(width) != "px" {
		return ""
	}
	v, ok := jsParseIntValue(width)
	if !ok {
		return ""
	}
	innerPaddings := float64(c.shorthandAttrValue(c, constants.MJMLInnerPadding, sideLeft)) + float64(c.shorthandAttrValue(c, constants.MJMLInnerPadding, sideRight))
	borders := float64(c.shorthandBorderValue(c, constants.MJMLBorder, sideRight)) + float64(c.shorthandBorderValue(c, constants.MJMLBorder, sideLeft))
	return jsPixels(saturateInf(v) - innerPaddings - borders)
}

func (c *MJButtonComponent) GetTagName() string {
	return "mj-button"
}

// Render implements optimized Writer-based rendering for MJButtonComponent
func (c *MJButtonComponent) Render(w io.StringWriter) error {
	// Get text content - use GetMixedContent() to support HTML inside button
	// This preserves HTML tags like <strong>, <em>, etc. per MJML spec
	// (mj-button is an "ending tag" that can contain HTML code)
	textContent := c.Node.GetMixedContent()

	// Get attributes with proper default and global attribute resolution
	backgroundColor := c.resolveAttribute(c, constants.MJMLBackgroundColor)
	border := c.resolveAttribute(c, constants.MJMLBorder)
	borderRadius := c.resolveAttribute(c, constants.MJMLBorderRadius)
	innerPadding := c.resolveAttribute(c, constants.MJMLInnerPadding)
	target := c.resolveAttribute(c, constants.MJMLTarget)
	verticalAlign := c.resolveAttribute(c, constants.MJMLVerticalAlign)
	href := c.resolveAttribute(c, constants.MJMLHref)
	width := c.resolveAttribute(c, constants.MJMLWidth)
	borderTop := c.resolveAttribute(c, constants.MJMLBorderTop)
	borderRight := c.resolveAttribute(c, constants.MJMLBorderRight)
	borderBottom := c.resolveAttribute(c, constants.MJMLBorderBottom)
	borderLeft := c.resolveAttribute(c, constants.MJMLBorderLeft)
	height := c.resolveAttribute(c, constants.MJMLHeight)

	// Font-related attributes (needed by both td and content elements)
	fontFamily := c.resolveAttribute(c, constants.MJMLFontFamily)
	fontSize := c.resolveAttribute(c, constants.MJMLFontSize)
	fontWeight := c.resolveAttribute(c, constants.MJMLFontWeight)
	fontStyle := c.resolveAttribute(c, constants.MJMLFontStyle)
	color := c.resolveAttribute(c, constants.MJMLColor)
	lineHeight := c.resolveAttribute(c, constants.MJMLLineHeight)
	textDecoration := c.resolveAttribute(c, constants.MJMLTextDecoration)
	textTransform := c.resolveAttribute(c, constants.MJMLTextTransform)

	// Determine if we use <a> or <p> tag
	tagName := "p"
	if href != "" {
		tagName = "a"
	}

	// Button table structure
	tableTag := html.NewHTMLTag("table")
	c.addDebugAttribute(tableTag, "button")
	tableTag.
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrRole, "presentation").
		AddStyle(constants.CSSBorderCollapse, constants.BorderCollapseSeparate)

	// Add width to table if specified
	if width != "" {
		tableTag.AddStyle(constants.CSSWidth, width)
	}

	tableTag.AddStyle(constants.CSSLineHeight, "100%")

	if err := tableTag.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody><tr>"); err != nil {
		return err
	}

	// Button cell with background and border styles
	buttonTdTag := html.NewHTMLTag("td").
		AddAttribute(constants.AttrAlign, constants.AlignCenter)
	// As mjml-button: no bgcolor for background-color="none".
	if backgroundColor != "none" {
		buttonTdTag.AddAttribute(constants.AttrBgcolor, backgroundColor)
	}
	buttonTdTag.AddAttribute(constants.AttrRole, "presentation").
		AddAttribute(constants.AttrValign, verticalAlign).
		AddStyle(constants.CSSBorder, border)

	if borderBottom != "" {
		buttonTdTag.AddStyle(constants.CSSBorderBottom, borderBottom)
	}
	if borderLeft != "" {
		buttonTdTag.AddStyle(constants.CSSBorderLeft, borderLeft)
	}

	buttonTdTag.AddStyle(constants.CSSBorderRadius, borderRadius)

	if borderRight != "" {
		buttonTdTag.AddStyle(constants.CSSBorderRight, borderRight)
	}
	if borderTop != "" {
		buttonTdTag.AddStyle(constants.CSSBorderTop, borderTop)
	}

	buttonTdTag.AddStyle(constants.CSSCursor, "auto")
	if height != "" {
		buttonTdTag.AddStyle(constants.CSSHeight, height)
	}
	if fontStyle != "" {
		buttonTdTag.AddStyle(constants.CSSFontStyle, fontStyle)
	}
	buttonTdTag.AddStyle(constants.CSSMSOPaddingAlt, innerPadding)
	if c.hasAttribute(c, constants.MJMLTextAlign) {
		buttonTdTag.AddStyle(constants.CSSTextAlign, c.resolveAttribute(c, constants.MJMLTextAlign))
	}
	buttonTdTag.AddStyle(constants.CSSBackground, backgroundColor)

	if err := buttonTdTag.RenderOpen(w); err != nil {
		return err
	}

	// Button content (a or p tag)
	// As mjml-button's content tag: href, name, rel and title as set (an
	// empty href too, on the p MJML writes for it), target on a link only.
	contentTag := html.NewHTMLTag(tagName)
	for _, name := range [...]string{constants.MJMLHref, constants.MJMLName, constants.MJMLRel, constants.MJMLTitle} {
		c.addSetAttribute(c, contentTag, name)
	}
	if href != "" && target != "" {
		contentTag.AddAttribute(constants.AttrTarget, target)
	}

	// Calculate inner width for anchor tag
	innerWidth := c.linkWidth(width)

	// Apply button content styles in MJML order
	contentTag.AddStyle(constants.CSSDisplay, constants.DisplayInlineBlock)

	// Add width if calculated
	if innerWidth != "" {
		contentTag.AddStyle(constants.CSSWidth, innerWidth)
	}

	contentTag.AddStyle(constants.CSSBackground, backgroundColor).
		AddStyle(constants.CSSColor, color).
		AddStyle(constants.CSSFontFamily, fontFamily).
		AddStyle(constants.CSSFontSize, fontSize)

	if fontStyle != "" {
		contentTag.AddStyle(constants.CSSFontStyle, fontStyle)
	}

	contentTag.AddStyle(constants.CSSFontWeight, fontWeight).
		AddStyle(constants.CSSLineHeight, lineHeight)
	if c.hasAttribute(c, constants.MJMLLetterSpacing) {
		contentTag.AddStyle(constants.CSSLetterSpacing, c.resolveAttribute(c, constants.MJMLLetterSpacing))
	}
	contentTag.AddStyle(constants.CSSMargin, "0").
		AddStyle(constants.CSSTextDecoration, textDecoration).
		AddStyle(constants.CSSTextTransform, textTransform).
		AddStyle(constants.CSSPadding, innerPadding).
		AddStyle(constants.CSSMSOPaddingAlt, "0px").
		AddStyle(constants.CSSBorderRadius, borderRadius)

	if err := contentTag.RenderOpen(w); err != nil {
		return err
	}
	if err := c.writeBlank(w); err != nil {
		return err
	}
	if _, err := w.WriteString(textContent); err != nil {
		return err
	}
	if err := contentTag.RenderClose(w); err != nil {
		return err
	}
	if err := buttonTdTag.RenderClose(w); err != nil {
		return err
	}
	if _, err := w.WriteString("</tr></tbody>"); err != nil {
		return err
	}
	if err := tableTag.RenderClose(w); err != nil {
		return err
	}

	return nil
}

func (c *MJButtonComponent) GetDefaultAttribute(name string) string {
	switch name {
	case "align":
		return "center"
	case "background-color":
		return "#414141"
	case "border":
		return "none"
	case "border-radius":
		return "3px"
	case "color":
		return "#ffffff"
	case "font-family":
		return fonts.DefaultFontStack
	case "font-size":
		return "13px"
	case "font-weight":
		return "normal"
	case "inner-padding":
		return "10px 25px"
	case "line-height":
		return "120%"
	case "padding":
		return "10px 25px"
	case "target":
		return "_blank"
	case "text-decoration":
		return "none"
	case "text-transform":
		return "none"
	case "vertical-align":
		return "middle"
	case "href":
		return ""
	default:
		return ""
	}
}
