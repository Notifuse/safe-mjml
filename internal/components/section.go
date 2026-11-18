package components

import (
	"io"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJSectionComponent represents mj-section
type MJSectionComponent struct {
	*BaseComponent
}

// NewMJSectionComponent creates a new mj-section component
func NewMJSectionComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJSectionComponent {
	return &MJSectionComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJSectionComponent) GetTagName() string {
	return "mj-section"
}

// Render writes the section as mjml-section does (sectionBlock), its
// children in one Outlook row.
func (c *MJSectionComponent) Render(w io.StringWriter) error {
	return sectionBlock{BaseComponent: c.BaseComponent, comp: c, wrapped: c.renderWrappedChildren}.render(w)
}

// renderWrappedChildren writes the children as mjml-section's
// renderWrappedChildren: one Outlook row, a cell around each child, a raw
// child as it is.
func (c *MJSectionComponent) renderWrappedChildren(w io.StringWriter) error {
	if err := html.RenderMSOConditional(w, "<tr>"); err != nil {
		return err
	}
	for _, child := range c.children {
		if child.IsRawElement() {
			if err := child.Render(w); err != nil {
				return err
			}
			continue
		}
		td := html.NewHTMLTag("td")
		if align, set := attributeOf(child, constants.MJMLAlign); set {
			td.AddAttribute(constants.AttrAlign, align)
		}
		css, _ := attributeOf(child, constants.MJMLCSSClass)
		td.AddAttribute(constants.AttrClass, suffixCSSClasses(css, constants.ClassSuffixOutlook))
		// The child's tdOutlook style: that of a column or a group.
		switch v := child.(type) {
		case *MJColumnComponent:
			td.AddStyle(constants.CSSVerticalAlign, v.resolveAttribute(v, constants.MJMLVerticalAlign)).
				AddStyle(constants.CSSWidth, v.GetWidthAsPixel())
			if v.hasColumnGutter() {
				td.AddStyle(constants.CSSPadding, v.outlookGutterCSS())
			}
		case *MJGroupComponent:
			if _, verticalAlign, set := v.msoCellStyle(); set {
				td.AddStyle(constants.CSSVerticalAlign, verticalAlign)
			}
			td.AddStyle(constants.CSSWidth, v.GetWidthAsPixel())
		default:
			// Any other child (a misplaced mj-text…) has no tdOutlook style:
			// an empty one, which the minifier keeps inside Outlook.
			td.AddAttribute(constants.AttrStyle, "")
		}
		if err := renderMSOTagOpen(w, td); err != nil {
			return err
		}
		if err := child.Render(w); err != nil {
			return err
		}
		if err := html.RenderMSOConditional(w, "</td>"); err != nil {
			return err
		}
	}
	return html.RenderMSOConditional(w, "</tr>")
}

// attributeOf returns an attribute of a component as resolveAttribute gives
// it, and whether a layer or the default sets it (getAttribute(name) being
// defined in MJML); "", false for a component without attributes.
func attributeOf(comp Component, name string) (string, bool) {
	c, ok := comp.(interface {
		resolveAttribute(Component, string) string
		hasAttribute(Component, string) bool
	})
	if !ok || !c.hasAttribute(comp, name) {
		return "", false
	}
	return c.resolveAttribute(comp, name), true
}

// resolvedOf returns an attribute of any component as MJML's getAttribute
// gives it, its default included, or "" for none.
func resolvedOf(comp Component, name string) string {
	if c, ok := comp.(interface {
		resolveAttribute(Component, string) string
	}); ok {
		return c.resolveAttribute(comp, name)
	}
	return ""
}

// renderMSOTagOpen writes an Outlook-only opening tag, with a space before
// its closing ">" as MJML writes it.
func renderMSOTagOpen(w io.StringWriter, td *html.HTMLTag) error {
	var open strings.Builder
	if err := td.RenderOpen(&open); err != nil {
		return err
	}
	return html.RenderMSOConditional(w, strings.TrimSuffix(open.String(), ">")+" >")
}

// sectionBlock renders what mjml-section and mjml-wrapper, which extends it,
// share: render, renderSimple, renderFullWidth, renderBefore, renderSection,
// renderAfter and renderWithBackground. Only the way the children are
// wrapped (renderWrappedChildren) differs between them. Every block writes
// its own Outlook conditional comments; the final merge
// (html.OutlookMergeWriter) joins the adjacent ones, as mjml-core does.
type sectionBlock struct {
	*BaseComponent
	comp    Component
	wrapped func(io.StringWriter) error
}

// attr returns the component's attribute and whether a layer or the default
// sets it.
func (b sectionBlock) attr(name string) (string, bool) {
	return attributeOf(b.comp, name)
}

func (b sectionBlock) value(name string) string {
	return b.resolveAttribute(b.comp, name)
}

func (b sectionBlock) isFullWidth() bool {
	return b.value(constants.MJMLFullWidth) == constants.MJMLFullWidth
}

func (b sectionBlock) hasBackground() bool {
	return b.hasAttribute(b.comp, constants.MJMLBackgroundUrl)
}

func (b sectionBlock) hasBorderRadius() bool {
	return b.value(constants.MJMLBorderRadius) != ""
}

// isFirst reports whether the block is the first child of its parent, raw
// children and comments included, as props.index === 0 in MJML.
func (b sectionBlock) isFirst() bool {
	return b.ctx.index == 0
}

// containerWidth is the width the block lays out in, as MJML's
// containerWidth string writes it (it can carry a fraction).
func (b sectionBlock) containerWidth() string {
	return b.containerWidthCSS()
}

func (b sectionBlock) background() backgroundImage {
	return resolveBackgroundImage(b.value(constants.MJMLBackgroundColor), b.value(constants.MJMLBackgroundUrl),
		b.value(constants.MJMLBackgroundPosition), b.value(constants.MJMLBackgroundPositionX),
		b.value(constants.MJMLBackgroundPositionY), b.value(constants.MJMLBackgroundSize),
		b.value(constants.MJMLBackgroundRepeat))
}

// addBackgroundStyles writes mjml-section's background styles: the image
// ones when background-url is not empty, the background color otherwise.
func (b sectionBlock) addBackgroundStyles(tag *html.HTMLTag) {
	if b.hasBackground() && b.background().applyCSS(tag) {
		return
	}
	if color, set := b.attr(constants.MJMLBackgroundColor); set {
		tag.AddStyle(constants.CSSBackground, color).
			AddStyle(constants.CSSBackgroundColor, color)
	}
}

func (b sectionBlock) render(w io.StringWriter) error {
	if b.isFullWidth() {
		return b.renderFullWidth(w)
	}
	return b.renderSimple(w)
}

func (b sectionBlock) renderSimple(w io.StringWriter) error {
	if err := b.renderBefore(w); err != nil {
		return err
	}
	if b.hasBackground() {
		if err := b.renderWithBackground(w, b.renderSection); err != nil {
			return err
		}
	} else if err := b.renderSection(w); err != nil {
		return err
	}
	return b.renderAfter(w)
}

func (b sectionBlock) renderFullWidth(w io.StringWriter) error {
	table := html.NewTableTag().AddAttribute(constants.AttrAlign, constants.AlignCenter)
	// No empty class outside the Outlook conditionals: MJML's minifier
	// removes it there.
	if css := b.value(constants.MJMLCSSClass); css != "" {
		table.AddAttribute(constants.AttrClass, css)
	}
	if b.hasBackground() {
		table.AddAttribute(constants.AttrBackground, htmlEscape(b.value(constants.MJMLBackgroundUrl)))
	}
	b.addBackgroundStyles(table)
	table.AddStyle(constants.CSSWidth, "100%")
	if err := table.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody><tr><td>"); err != nil {
		return err
	}
	content := func(w io.StringWriter) error {
		if err := b.renderBefore(w); err != nil {
			return err
		}
		if err := b.renderSection(w); err != nil {
			return err
		}
		return b.renderAfter(w)
	}
	if b.hasBackground() {
		if err := b.renderWithBackground(w, content); err != nil {
			return err
		}
	} else if err := content(w); err != nil {
		return err
	}
	_, err := w.WriteString("</td></tr></tbody></table>")
	return err
}

// renderBefore writes the Outlook table the block stands in: centred, the
// container width, the gap of a parent wrapper above all but the first
// child, and the background color unless the parent has a gap.
func (b sectionBlock) renderBefore(w io.StringWriter) error {
	css, _ := b.attr(constants.MJMLCSSClass)
	style := []html.StyleProperty{{Name: constants.CSSWidth, Value: b.containerWidth()}}
	if !b.isFirst() && b.ctx.gap != "" {
		style = append(style, html.StyleProperty{Name: constants.CSSPaddingTop, Value: b.ctx.gap})
	}
	// The style as an attribute, to keep mjml-section's attribute order
	// (HTMLTag writes AddStyle last).
	table := html.NewHTMLTag("table").
		AddAttribute(constants.AttrAlign, constants.AlignCenter).
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrClass, suffixCSSClasses(css, constants.ClassSuffixOutlook)).
		AddAttribute(constants.AttrRole, "presentation").
		AddAttribute(constants.AttrStyle, formatStyles(style)).
		AddAttribute(constants.AttrWidth, strconv.Itoa(b.GetEffectiveWidth()))
	if color := b.value(constants.MJMLBackgroundColor); color != "" && b.ctx.gap == "" {
		table.AddAttribute(constants.AttrBgcolor, color)
	}
	if err := renderMSOTagOpen(w, table); err != nil {
		return err
	}
	td := html.NewHTMLTag("td").
		AddStyle(constants.CSSLineHeight, "0px").
		AddStyle(constants.CSSFontSize, "0px").
		AddStyle(constants.CSSMSOLineHeightRule, "exactly")
	var open strings.Builder
	if err := td.RenderOpen(&open); err != nil {
		return err
	}
	return html.RenderMSOConditional(w, "<tr>"+open.String())
}

func (b sectionBlock) renderAfter(w io.StringWriter) error {
	return html.RenderMSOConditional(w, "</td></tr></table>")
}

// renderWithBackground wraps content in the VML rectangle that paints the
// background image in Outlook.
func (b sectionBlock) renderWithBackground(w io.StringWriter, content func(io.StringWriter) error) error {
	style := "width:" + b.containerWidth() + ";"
	if b.isFullWidth() {
		style = "mso-width-percent:1000;"
	}
	if err := html.RenderMSOConditional(w, b.background().vmlOpen(style)); err != nil {
		return err
	}
	if err := content(w); err != nil {
		return err
	}
	return html.RenderMSOConditional(w, "</v:textbox></v:rect>")
}

func (b sectionBlock) renderSection(w io.StringWriter) error {
	fullWidth := b.isFullWidth()
	div := html.NewHTMLTag("div")
	b.addDebugAttribute(div, b.comp.GetTagName())
	if css := b.value(constants.MJMLCSSClass); css != "" && !fullWidth {
		div.AddAttribute(constants.AttrClass, css)
	}
	if !fullWidth {
		b.addBackgroundStyles(div)
	}
	div.AddStyle(constants.CSSMargin, "0px auto").
		AddStyle(constants.CSSMaxWidth, b.containerWidth())
	if radius, set := b.attr(constants.MJMLBorderRadius); set {
		div.AddStyle(constants.CSSBorderRadius, radius)
	}
	if b.hasBorderRadius() {
		div.AddStyle(constants.CSSOverflow, "hidden")
	}
	if !b.isFirst() && b.ctx.gap != "" {
		div.AddStyle(constants.CSSMarginTop, b.ctx.gap)
	}
	if err := div.RenderOpen(w); err != nil {
		return err
	}
	var innerDiv *html.HTMLTag
	if b.hasBackground() {
		innerDiv = newBackgroundInnerDiv()
		if err := innerDiv.RenderOpen(w); err != nil {
			return err
		}
	}

	table := html.NewTableTag().AddAttribute(constants.AttrAlign, constants.AlignCenter)
	if !fullWidth {
		if b.hasBackground() {
			table.AddAttribute(constants.AttrBackground, htmlEscape(b.value(constants.MJMLBackgroundUrl)))
		}
		b.addBackgroundStyles(table)
	}
	table.AddStyle(constants.CSSWidth, "100%")
	if b.hasBorderRadius() {
		table.AddStyle(constants.CSSBorderCollapse, constants.BorderCollapseSeparate)
	}
	if err := table.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody><tr>"); err != nil {
		return err
	}

	// mjml-section's td style, in its order, each style when its attribute
	// is set.
	td := html.NewHTMLTag("td")
	for _, style := range []struct{ attr, css string }{
		{constants.MJMLBorder, constants.CSSBorder},
		{constants.MJMLBorderBottom, constants.CSSBorderBottom},
		{constants.MJMLBorderLeft, constants.CSSBorderLeft},
		{constants.MJMLBorderRight, constants.CSSBorderRight},
		{constants.MJMLBorderTop, constants.CSSBorderTop},
		{constants.MJMLBorderRadius, constants.CSSBorderRadius},
		{constants.MJMLDirection, constants.CSSDirection},
	} {
		if value, set := b.attr(style.attr); set {
			td.AddStyle(style.css, value)
		}
	}
	td.AddStyle(constants.CSSFontSize, "0px")
	for _, style := range []struct{ attr, css string }{
		{constants.MJMLPadding, constants.CSSPadding},
		{constants.MJMLPaddingBottom, constants.CSSPaddingBottom},
		{constants.MJMLPaddingLeft, constants.CSSPaddingLeft},
		{constants.MJMLPaddingRight, constants.CSSPaddingRight},
		{constants.MJMLPaddingTop, constants.CSSPaddingTop},
		{constants.MJMLTextAlign, constants.CSSTextAlign},
	} {
		if value, set := b.attr(style.attr); set {
			td.AddStyle(style.css, value)
		}
	}
	if err := td.RenderOpen(w); err != nil {
		return err
	}
	if err := b.writeBlank(w); err != nil {
		return err
	}
	if err := html.RenderMSOConditional(w, `<table role="presentation" border="0" cellpadding="0" cellspacing="0">`); err != nil {
		return err
	}
	if err := b.wrapped(w); err != nil {
		return err
	}
	if err := html.RenderMSOConditional(w, "</table>"); err != nil {
		return err
	}
	if _, err := w.WriteString("</td></tr></tbody></table>"); err != nil {
		return err
	}
	if innerDiv != nil {
		if err := innerDiv.RenderClose(w); err != nil {
			return err
		}
	}
	return div.RenderClose(w)
}

func (c *MJSectionComponent) GetDefaultAttribute(name string) string {
	switch name {
	case "background-color":
		return ""
	case constants.MJMLBackgroundUrl:
		return ""
	case "background-position":
		return "top center"
	case "background-position-x":
		return ""
	case "background-position-y":
		return ""
	case "background-repeat":
		return "repeat"
	case "background-size":
		return "auto"
	case "direction":
		return "ltr"
	case "full-width":
		return ""
	case "padding":
		return "20px 0"
	case "text-align":
		return "center"
	case "text-padding":
		return "4px 4px 4px 0"
	default:
		return ""
	}
}

// backgroundImage holds the resolved background attributes of an mj-section
// or mj-wrapper that has a background-url.
type backgroundImage struct {
	color, url, posX, posY, size, repeat string
}

// resolveBackgroundImage resolves the background position like mjml-section's
// getBackgroundPosition: background-position split in x and y, then
// overridden by background-position-x and background-position-y.
func resolveBackgroundImage(color, url, position, positionX, positionY, size, repeat string) backgroundImage {
	posX, posY := parseBackgroundPosition(position)
	posX, posY = overridePosition(posX, posY, positionX, positionY)
	return backgroundImage{color: color, url: url, posX: posX, posY: posY, size: size, repeat: repeat}
}

// vmlOpen returns the Outlook markup opening the VML rectangle that paints the
// background image: v:rect with the given style, v:fill and v:textbox. The
// color attribute is only written when a background-color is set.
func (b backgroundImage) vmlOpen(rectStyle string) string {
	vOriginX, vOriginY, vPosX, vPosY := computeVMLPosition(b.posX, b.posY, b.size, b.repeat)
	vSizeAttrs, vAspect := computeVMLSize(b.size)
	vmlType := computeVMLType(b.repeat, b.size)

	var sb strings.Builder
	sb.WriteString(`<v:rect style="`)
	sb.WriteString(htmlEscape(rectStyle))
	sb.WriteString(`" xmlns:v="urn:schemas-microsoft-com:vml" fill="true" stroke="false"><v:fill origin="`)
	sb.WriteString(htmlEscape(vOriginX + `, ` + vOriginY))
	sb.WriteString(`" position="`)
	sb.WriteString(htmlEscape(vPosX + `, ` + vPosY))
	sb.WriteString(`" src="`)
	sb.WriteString(htmlEscape(b.url))
	sb.WriteString(`"`)
	if b.color != "" {
		sb.WriteString(` color="` + htmlEscape(b.color) + `"`)
	}
	sb.WriteString(` type="` + vmlType + `"`)
	if vSizeAttrs != "" {
		sb.WriteString(" " + vSizeAttrs)
	}
	if vAspect != "" {
		sb.WriteString(` aspect="` + vAspect + `"`)
	}
	sb.WriteString(` /><v:textbox style="mso-fit-shape-to-text:true" inset="0,0,0,0">`)
	return sb.String()
}

// applyCSS sets the background shorthand and the position, repeat and size
// longhands MJML writes next to it. It reports false, setting nothing, when
// the URL is empty, where mjml-section writes the background color instead
// (its CSS tests getAttribute('background-url'), its VML hasBackground), or
// when the shorthand is empty.
func (b backgroundImage) applyCSS(tag *html.HTMLTag) bool {
	if b.url == "" {
		return false
	}
	shorthand := buildBackgroundShorthand(b.color, b.url, b.posX, b.posY, b.size, b.repeat)
	if shorthand == "" {
		return false
	}
	tag.AddStyle(constants.CSSBackground, shorthand).
		AddStyle(constants.CSSBackgroundPosition, b.posX+" "+b.posY).
		AddStyle(constants.CSSBackgroundRepeat, b.repeat).
		AddStyle(constants.CSSBackgroundSize, b.size)
	return true
}

// newBackgroundInnerDiv returns the div MJML puts inside a section or wrapper
// with a background image (innerDiv in mjml-section), so the image does not
// add line height. font-size is a unitless 0, as in MJML.
func newBackgroundInnerDiv() *html.HTMLTag {
	return html.NewHTMLTag("div").
		AddStyle(constants.CSSLineHeight, "0").
		AddStyle(constants.CSSFontSize, "0")
}

// childContainerWidth returns the width the section gives its columns and
// groups: its width minus its left and right padding and borders, as
// mjml-section's getChildContext computes it (getBoxWidths), negative when
// the padding is wider than the section, as in MJML.
// childContext gives it to the children.
func (c *MJSectionComponent) childContainerWidth() int {
	return c.boxWidth(c)
}

// formatStyles writes style declarations as the value of a style attribute
// ("name:value;" each, in order), as HTMLTag writes those of AddStyle.
func formatStyles(styles []html.StyleProperty) string {
	var b strings.Builder
	for _, style := range styles {
		b.WriteString(style.Name + ":" + style.Value + ";")
	}
	return b.String()
}
