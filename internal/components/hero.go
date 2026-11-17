package components

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJHeroComponent represents the mj-hero component
type MJHeroComponent struct {
	*BaseComponent
}

func NewMJHeroComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJHeroComponent {
	return &MJHeroComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

// Render writes the hero as mjml-hero's render does: an Outlook table with
// the VML background image, then the div, table and row around the cell
// renderMode writes.
func (c *MJHeroComponent) Render(w io.StringWriter) error {
	// The container width as MJML's containerWidth string writes it, with
	// its fraction (600.5px); only the outer table's width attribute is
	// parseInt of it.
	containerWidthPx := c.containerWidthCSS()
	containerWidth := c.GetEffectiveWidth()

	if _, err := w.WriteString("<!--[if mso | IE]>"); err != nil {
		return err
	}
	msoTable := fmt.Sprintf(`<table align="center" border="0" cellpadding="0" cellspacing="0" role="presentation" style="width:%s;" width="%d" ><tr><td style="line-height:0;font-size:0;mso-line-height-rule:exactly;">`, containerWidthPx, containerWidth)
	if _, err := w.WriteString(msoTable); err != nil {
		return err
	}
	if err := c.outlookImage(containerWidthPx).RenderSelfClosing(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<![endif]-->"); err != nil {
		return err
	}

	div := html.NewHTMLTag("div")
	// align is not an allowed attribute of mj-hero, but mjml-hero reads it.
	if align, set := attributeOf(c, constants.MJMLAlign); set {
		div.AddAttribute(constants.AttrAlign, align)
	}
	div.AddStyle(constants.CSSMargin, "0 auto").
		AddStyle(constants.CSSMaxWidth, containerWidthPx)
	c.setClassAttribute(div)
	if err := div.RenderOpen(w); err != nil {
		return err
	}
	table := html.NewHTMLTag("table").
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrRole, "presentation").
		AddStyle(constants.CSSWidth, "100%")
	if err := table.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody>"); err != nil {
		return err
	}
	tr := html.NewHTMLTag("tr").AddStyle(constants.CSSVerticalAlign, constants.VAlignTop)
	if err := tr.RenderOpen(w); err != nil {
		return err
	}

	if err := c.renderMode(w, containerWidthPx); err != nil {
		return err
	}

	if _, err := w.WriteString("</tr></tbody></table></div>"); err != nil {
		return err
	}
	_, err := w.WriteString("<!--[if mso | IE]></td></tr></table><![endif]-->")
	return err
}

// outlookImage returns the VML image of the Outlook table, with mjml-hero's
// outlook-image style. As in MJML, an attribute set to the empty string is
// written empty (height:; src=""), and only an unset one is left out.
func (c *MJHeroComponent) outlookImage(containerWidthPx string) *html.HTMLTag {
	style := []html.StyleProperty{{Name: constants.CSSBorder, Value: "0"}}
	if height, set := attributeOf(c, constants.MJMLBackgroundHeight); set {
		style = append(style, html.StyleProperty{Name: constants.CSSHeight, Value: height})
	}
	// background-width || containerWidth: "0px" is a width, "" is not.
	width := c.resolveAttribute(c, constants.MJMLBackgroundWidth)
	if width == "" {
		width = containerWidthPx
	}
	style = append(style,
		html.StyleProperty{Name: "mso-position-horizontal", Value: "center"},
		html.StyleProperty{Name: "position", Value: "absolute"},
		html.StyleProperty{Name: "top", Value: "0"},
		html.StyleProperty{Name: constants.CSSWidth, Value: width},
		html.StyleProperty{Name: "z-index", Value: "-3"})

	image := html.NewHTMLTag("v:image").AddAttribute(constants.AttrStyle, formatStyles(style))
	if url, set := attributeOf(c, constants.MJMLBackgroundUrl); set {
		image.AddAttribute("src", url)
	}
	return image.AddAttribute("xmlns:v", "urn:schemas-microsoft-com:vml")
}

// background returns mjml-hero's getBackground: the background color, then,
// with a background URL, the image, no-repeat and the position with cover;
// the empty parts left out.
func (c *MJHeroComponent) background() string {
	var parts []string
	if color := c.resolveAttribute(c, constants.MJMLBackgroundColor); color != "" {
		parts = append(parts, color)
	}
	if url := c.resolveAttribute(c, constants.MJMLBackgroundUrl); url != "" {
		parts = append(parts, "url('"+url+"')", constants.BackgroundRepeatNoRepeat,
			c.resolveAttribute(c, constants.MJMLBackgroundPosition)+" / cover")
	}
	return strings.Join(parts, " ")
}

// renderMode writes the hero's cell as mjml-hero's renderMode does, with
// its commonAttributes, and the content in it: in the fixed-height mode,
// the default, with its height; in the fluid-height mode, without it,
// between two cells whose bottom padding keeps the background image's
// ratio. As in MJML, an attribute set to the empty string is written empty,
// and only an unset one is left out.
func (c *MJHeroComponent) renderMode(w io.StringWriter, containerWidthPx string) error {
	fluid := c.resolveAttribute(c, constants.MJMLMode) == "fluid-height"
	if fluid {
		if err := c.renderFluidCell(w); err != nil {
			return err
		}
	}
	td := html.NewHTMLTag("td")
	if url, set := attributeOf(c, constants.MJMLBackgroundUrl); set {
		td.AddAttribute(constants.AttrBackground, url)
	}
	td.AddStyle(constants.CSSBackground, c.background())
	if position, set := attributeOf(c, constants.MJMLBackgroundPosition); set {
		td.AddStyle(constants.CSSBackgroundPosition, position)
	}
	td.AddStyle(constants.CSSBackgroundRepeat, constants.BackgroundRepeatNoRepeat)
	if radius, set := attributeOf(c, constants.MJMLBorderRadius); set {
		td.AddStyle(constants.CSSBorderRadius, radius)
	}
	for _, side := range []struct{ attr, css string }{
		{constants.MJMLPadding, constants.CSSPadding},
		{constants.MJMLPaddingTop, constants.CSSPaddingTop},
		{constants.MJMLPaddingLeft, constants.CSSPaddingLeft},
		{constants.MJMLPaddingRight, constants.CSSPaddingRight},
		{constants.MJMLPaddingBottom, constants.CSSPaddingBottom},
	} {
		if value, set := attributeOf(c, side.attr); set {
			td.AddStyle(side.css, value)
		}
	}
	if verticalAlign, set := attributeOf(c, constants.MJMLVerticalAlign); set {
		td.AddStyle(constants.CSSVerticalAlign, verticalAlign)
	}

	if !fluid {
		effectiveHeight := c.calculateEffectiveHeight(c.resolveAttribute(c, constants.MJMLHeight))
		td.AddAttribute(constants.AttrHeight, strings.TrimSuffix(effectiveHeight, "px")).
			AddStyle(constants.CSSHeight, effectiveHeight)
	}

	if err := td.RenderOpen(w); err != nil {
		return err
	}
	if err := c.renderContent(w, containerWidthPx); err != nil {
		return err
	}
	if _, err := w.WriteString("</td>"); err != nil {
		return err
	}
	if fluid {
		return c.renderFluidCell(w)
	}
	return nil
}

// renderFluidCell writes one of the cells around the hero's cell in the
// fluid-height mode (mjml-hero's td-fluid), whose bottom padding is the
// ratio of background-height to background-width, rounded, in percent.
// MJML writes it as an opening tag, "<td ... />" that its minifier keeps
// without a closing tag. Without a ratio (a missing, non-numeric or zero
// background width or height), MJML writes padding-bottom:NaN% (or
// Infinity%), and safe-mjml no padding-bottom (Pierre's rule).
func (c *MJHeroComponent) renderFluidCell(w io.StringWriter) error {
	td := html.NewHTMLTag("td").AddStyle(constants.CSSWidth, "0.01%")
	backgroundHeight, heightOK := leadingInt(c.resolveAttribute(c, constants.MJMLBackgroundHeight))
	backgroundWidth, widthOK := leadingInt(c.resolveAttribute(c, constants.MJMLBackgroundWidth))
	if heightOK && widthOK && backgroundWidth != 0 {
		// Math.round: halves round up.
		ratio := math.Floor(float64(backgroundHeight)/float64(backgroundWidth)*100 + 0.5)
		td.AddStyle(constants.CSSPaddingBottom, strconv.FormatFloat(ratio, 'f', -1, 64)+"%")
	}
	td.AddStyle(constants.CSSMSOPaddingBottomAlt, "0")
	return td.RenderOpen(w)
}

// renderContent writes what mjml-hero's renderContent does: an Outlook
// table, the mj-hero-content div, and the tables around the children, each
// in its row and cell.
func (c *MJHeroComponent) renderContent(w io.StringWriter, containerWidthPx string) error {
	// The Outlook inner table and cell (outlook-inner-table and
	// outlook-inner-td), with the inner background and padding.
	innerWidth := c.childContainerWidth()
	align, alignSet := attributeOf(c, constants.MJMLAlign)
	outlookTable := html.NewHTMLTag("table")
	if alignSet {
		outlookTable.AddAttribute(constants.AttrAlign, align)
	}
	outlookTable.AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		// The width the hero gives its children (outlook-inner-table), and
		// parseInt of it, as mjml-hero writes them since MJML 5.
		AddAttribute(constants.AttrStyle, "width:"+jsPixels(innerWidth)+";").
		AddAttribute(constants.AttrWidth, jsParseInt(jsNumber(innerWidth)))
	var outlookTableOpen strings.Builder
	if err := outlookTable.RenderOpen(&outlookTableOpen); err != nil {
		return err
	}
	innerBackground, innerBackgroundSet := attributeOf(c, constants.MJMLInnerBackgroundColor)
	var tdStyle []html.StyleProperty
	if innerBackgroundSet {
		tdStyle = append(tdStyle, html.StyleProperty{Name: constants.CSSBackgroundColor, Value: innerBackground})
	}
	innerPadding := c.innerPadding()
	tdStyle = append(tdStyle, innerPadding...)
	outlookInner := "<!--[if mso | IE]>" + strings.TrimSuffix(outlookTableOpen.String(), ">") + ` ><tr><td style="` + html.EscapeAttribute(formatStyles(tdStyle)) + `"><![endif]-->`
	if _, err := w.WriteString(outlookInner); err != nil {
		return err
	}

	content := html.NewHTMLTag("div")
	if alignSet {
		content.AddAttribute(constants.AttrAlign, align)
	}
	content.AddAttribute(constants.AttrClass, "mj-hero-content")
	if innerBackgroundSet {
		content.AddStyle(constants.CSSBackgroundColor, innerBackground)
	}
	if alignSet {
		content.AddStyle(constants.CSSFloat, align)
	}
	content.AddStyle(constants.CSSMargin, "0px auto")
	if width, set := attributeOf(c, constants.MJMLWidth); set {
		content.AddStyle(constants.CSSWidth, width)
	}
	// The inner padding applies in every client since MJML 5 (inner-div),
	// not only in Outlook's cell.
	for _, p := range innerPadding {
		content.AddStyle(p.Name, p.Value)
	}
	if err := content.RenderOpen(w); err != nil {
		return err
	}
	if err := innerTable().RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody><tr><td>"); err != nil {
		return err
	}
	if err := innerTable().RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody>"); err != nil {
		return err
	}
	// The children, each in the row and cell mjml-hero gives it, with a
	// background attribute too.
	for _, child := range c.children {
		if err := renderInCell(w, child, true); err != nil {
			return err
		}
	}
	if _, err := w.WriteString("</tbody></table></td></tr></tbody></table></div>"); err != nil {
		return err
	}
	_, err := w.WriteString("<!--[if mso | IE]></td></tr></table><![endif]-->")
	return err
}

// innerPadding returns the inner-padding attributes that are set, the
// empty string included, as styles, in mjml-hero's order.
func (c *MJHeroComponent) innerPadding() []html.StyleProperty {
	var styles []html.StyleProperty
	for _, side := range []struct{ attr, css string }{
		{constants.MJMLInnerPadding, constants.CSSPadding},
		{constants.MJMLInnerPaddingTop, constants.CSSPaddingTop},
		{constants.MJMLInnerPaddingLeft, constants.CSSPaddingLeft},
		{constants.MJMLInnerPaddingRight, constants.CSSPaddingRight},
		{constants.MJMLInnerPaddingBottom, constants.CSSPaddingBottom},
	} {
		if value, set := attributeOf(c, side.attr); set {
			styles = append(styles, html.StyleProperty{Name: side.css, Value: value})
		}
	}
	return styles
}

// innerTable returns the opening tag of mjml-hero's inner-table.
func innerTable() *html.HTMLTag {
	return html.NewHTMLTag("table").
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrRole, "presentation").
		AddStyle(constants.CSSWidth, "100%").
		AddStyle(constants.CSSMargin, "0px")
}

func (c *MJHeroComponent) GetTagName() string {
	return "mj-hero"
}

// calculateEffectiveHeight returns the height of the hero's cell, as
// mjml-hero's renderMode computes it: parseInt(height) minus the top and
// bottom padding (getShorthandAttrValue), with no floor, so that a padding
// taller than the height gives a negative height, as in MJML. A height that
// does not start with a number, the empty one included, is kept as written:
// MJML writes NaNpx there, which safe-mjml never writes (Pierre's rule).
func (c *MJHeroComponent) calculateEffectiveHeight(height string) string {
	heightVal, ok := leadingInt(height)
	if !ok {
		return height
	}
	return fmt.Sprintf("%dpx", sumLengths(heightVal,
		-c.shorthandAttrValue(c, constants.MJMLPadding, sideTop),
		-c.shorthandAttrValue(c, constants.MJMLPadding, sideBottom)))
}

func (c *MJHeroComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLBackgroundColor:
		return "#ffffff"
	case constants.MJMLBackgroundPosition:
		return "center center"
	case constants.MJMLHeight:
		return "0px"
	case constants.MJMLMode:
		return "fixed-height"
	case constants.MJMLPadding:
		return "0px"
	case constants.MJMLVerticalAlign:
		return constants.VAlignTop
	default:
		return ""
	}
}

// childContainerWidth returns the width the hero gives its children, as
// mjml-hero's getChildContext computes it since MJML 5: parseFloat of its
// container width, its fraction kept (600.5px), minus its left and right
// padding. Borders are not subtracted, as in MJML. A padding wider than the
// hero gives 0, where MJML gives a negative width (Pierre's rule: no
// negative container width).
func (c *MJHeroComponent) childContainerWidth() float64 {
	padding := sumLengths(c.shorthandAttrValue(c, constants.MJMLPadding, sideLeft),
		c.shorthandAttrValue(c, constants.MJMLPadding, sideRight))
	return max(c.heroContainerWidth()-float64(padding), 0)
}

// heroContainerWidth is parseFloat(containerWidth) in mjml-hero's
// getChildContext: the container width as its string writes it, read as
// parseFloat reads it (a body width of 1e2px is 100 pixels).
func (c *MJHeroComponent) heroContainerWidth() float64 {
	if width, ok := jsParseFloat(c.containerWidthCSS()); ok && !math.IsInf(width, 0) {
		return width
	}
	return c.BaseComponent.layoutWidth()
}
