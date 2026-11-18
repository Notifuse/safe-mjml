package components

import (
	"io"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJImageComponent represents mj-image
type MJImageComponent struct {
	*BaseComponent
}

// NewMJImageComponent creates a new mj-image component
func NewMJImageComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJImageComponent {
	return &MJImageComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJImageComponent) GetTagName() string {
	return "mj-image"
}

// Render implements optimized Writer-based rendering for MJImageComponent
func (c *MJImageComponent) Render(w io.StringWriter) error {
	// Get attributes with proper resolution order (element -> class -> global -> default)
	border := c.resolveAttribute(c, constants.MJMLBorder)
	borderRadius := c.resolveAttribute(c, constants.MJMLBorderRadius)
	height := c.resolveAttribute(c, constants.MJMLHeight)
	href := c.resolveAttribute(c, constants.MJMLHref)
	rel := c.resolveAttribute(c, constants.MJMLRel)
	src := c.resolveAttribute(c, constants.MJMLSrc)
	target := c.resolveAttribute(c, constants.MJMLTarget)
	title := c.resolveAttribute(c, constants.MJMLTitle)

	width := getPixelWidthString(c.contentWidth())
	fluidOnMobile := c.resolveAttribute(c, constants.MJMLFluidOnMobile)
	// full-width is not an attribute mj-image allows (MJML warns too), but
	// mjml-image reads it: the table takes the width, at least and at most
	// 100%, and the cell none.
	fullWidth := c.resolveAttribute(c, constants.MJMLFullWidth) == constants.MJMLFullWidth

	// MJML always emits an alt attribute (falling back to the empty string when
	// not provided) to preserve accessibility defaults. Use the attribute
	// resolution pipeline so global attributes and mj-attributes blocks are
	// honoured.
	alt := c.resolveAttribute(c, constants.MJMLAlt)

	// Parse width to remove 'px' suffix for img width attribute
	imgWidth := width
	if before, ok := strings.CutSuffix(width, "px"); ok {
		imgWidth = before
	}

	// The img's height attribute, as mjml-image writes it: auto as is,
	// any other height through parseInt (120.7px gives 120).
	imgHeight := height
	if height != "auto" {
		imgHeight = parseIntText(height)
	}

	// Image table
	tableTag := html.NewHTMLTag("table").
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrRole, "presentation")
	if fullWidth {
		tableTag.AddStyle(constants.CSSMinWidth, "100%").
			AddStyle(constants.CSSMaxWidth, "100%").
			AddStyle(constants.CSSWidth, width)
	}
	tableTag.AddStyle(constants.CSSBorderCollapse, "collapse").
		AddStyle(constants.CSSBorderSpacing, "0px")

	if fluidOnMobile == "true" {
		tableTag.AddAttribute(constants.AttrClass, "mj-full-width-mobile")
	}

	if err := tableTag.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody><tr>"); err != nil {
		return err
	}

	// Image cell with width constraint
	imageTdTag := html.NewHTMLTag("td")
	if width != "" && !fullWidth {
		imageTdTag.AddStyle(constants.CSSWidth, width)
	}
	if fluidOnMobile == "true" {
		imageTdTag.AddAttribute(constants.AttrClass, "mj-full-width-mobile")
	}

	if err := imageTdTag.RenderOpen(w); err != nil {
		return err
	}

	// Optional link wrapper
	if href != "" {
		linkTag := html.NewHTMLTag("a").
			AddAttribute(constants.AttrHref, href)

		if rel != "" {
			linkTag.AddAttribute(constants.AttrRel, rel)
		}
		if target != "" {
			linkTag.AddAttribute(constants.AttrTarget, target)
		}
		// As mjml-image's link: its name and title too.
		for _, name := range [...]string{constants.MJMLName, constants.MJMLTitle} {
			c.addSetAttribute(c, linkTag, name)
		}

		if err := linkTag.RenderOpen(w); err != nil {
			return err
		}
	}

	// Image element with styles
	imgTag := html.NewHTMLTag("img")
	c.addDebugAttribute(imgTag, "image")

	// Set image attributes following MJML ordering.
	imgTag.AddAttribute("alt", alt)
	if imgHeight != "" {
		imgTag.AddAttribute(constants.AttrHeight, imgHeight)
	}
	// As mjml-image: src as resolved, even empty (src=""), and no src at all
	// when no layer sets one; MJML neither fails nor warns.
	if c.hasAttribute(c, constants.MJMLSrc) {
		imgTag.AddAttribute(constants.AttrSrc, src)
	}
	// srcset and sizes as set, as mjml-image writes them.
	for _, name := range [...]string{constants.AttrSrcset, constants.AttrSizes} {
		c.addSetAttribute(c, imgTag, name)
	}
	if title != "" {
		imgTag.AddAttribute(constants.AttrTitle, title)
	}
	if imgWidth != "" {
		imgTag.AddAttribute(constants.AttrWidth, imgWidth)
	}
	c.addSetAttribute(c, imgTag, constants.AttrUsemap)

	// Apply image styles: the border, then its sides, as mjml-image writes them
	imgTag.AddStyle(constants.CSSBorder, border)
	for _, side := range []string{constants.MJMLBorderLeft, constants.MJMLBorderRight, constants.MJMLBorderTop, constants.MJMLBorderBottom} {
		if value := c.resolveAttribute(c, side); value != "" {
			imgTag.AddStyle(side, value)
		}
	}
	imgTag.AddStyle(constants.CSSDisplay, "block").
		AddStyle(constants.CSSOutline, "none").
		AddStyle(constants.CSSTextDecoration, "none").
		AddStyle(constants.CSSHeight, height)
	if c.hasAttribute(c, constants.MJMLMaxHeight) {
		imgTag.AddStyle(constants.CSSMaxHeight, c.resolveAttribute(c, constants.MJMLMaxHeight))
	}
	if fullWidth {
		imgTag.AddStyle(constants.CSSMinWidth, "100%")
	}
	imgTag.AddStyle(constants.CSSWidth, "100%")
	if fullWidth {
		imgTag.AddStyle(constants.CSSMaxWidth, "100%")
	}
	imgTag.AddStyle(constants.CSSFontSize, c.resolveAttribute(c, constants.MJMLFontSize))

	if borderRadius != "" {
		imgTag.AddStyle(constants.CSSBorderRadius, borderRadius)
	}

	if err := imgTag.RenderOpen(w); err != nil {
		return err
	}

	// Close optional link wrapper
	if href != "" {
		if _, err := w.WriteString("</a>"); err != nil {
			return err
		}
	}

	if err := imageTdTag.RenderClose(w); err != nil {
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

func (c *MJImageComponent) GetDefaultAttribute(name string) string {
	switch name {
	case "align":
		return "center"
	case "alt":
		return ""
	case "border":
		return "0"
	case "border-radius":
		return ""
	case "font-size":
		return "13px"
	case "height":
		return "auto"
	case "href":
		return ""
	case "padding":
		return "10px 25px"
	case "rel":
		return ""
	case "src":
		return ""
	case "target":
		return "_blank"
	case "title":
		return ""
	case "fluid-on-mobile":
		return "false"
	case constants.MJMLContainerBackgroundColor:
		return ""
	default:
		return ""
	}
}

// contentWidth returns the width of the image, as mjml-image's
// getContentWidth computes it: its width attribute (mj-class and
// mj-attributes included), read like parseInt, but no wider than its box,
// the container width minus its left and right padding and borders
// (getBoxWidths); its box without a width. The box is never negative, but a
// negative width attribute is written as set (width="-37"), as MJML does
// (Pierre's decision of 2026-11-18): clients ignore it and show the image.
func (c *MJImageComponent) contentWidth() int {
	box := nonNegativeWidth(c.BaseComponent, c.boxWidth(c))
	if width, ok := leadingInt(c.resolveAttribute(c, constants.MJMLWidth)); ok {
		return min(width, box)
	}
	return box
}
