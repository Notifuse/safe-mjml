package components

import (
	"encoding/xml"
	"io"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/fonts"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJAccordionComponent represents the mj-accordion component
type MJAccordionComponent struct {
	*BaseComponent
}

func NewMJAccordionComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJAccordionComponent {
	return &MJAccordionComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJAccordionComponent) Render(w io.StringWriter) error {
	border := c.resolveAttribute(c, constants.MJMLBorder)
	fontFamily := c.resolveAttribute(c, constants.MJMLFontFamily)

	// Start the accordion table wrapper
	tableTag := html.NewHTMLTag("table").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrClass, "mj-accordion").
		AddStyle(constants.CSSWidth, "100%").
		AddStyle(constants.CSSBorderCollapse, constants.BorderCollapseCollapse).
		AddStyle(constants.CSSBorder, border).
		AddStyle(constants.CSSBorderBottom, "none").
		AddStyle(constants.CSSFontFamily, fontFamily)

	if err := tableTag.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody>"); err != nil {
		return err
	}

	// Every child in document order, as mjml-accordion renders it: the
	// elements, mj-raw, and a misplaced component (with a warning).
	for _, child := range c.children {
		if err := child.Render(w); err != nil {
			return err
		}
	}

	// Close the accordion table
	if _, err := w.WriteString("</tbody></table>"); err != nil {
		return err
	}

	return nil
}

func (c *MJAccordionComponent) GetTagName() string {
	return "mj-accordion"
}

func (c *MJAccordionComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLBorder:
		return "2px solid black"
	case constants.MJMLFontFamily:
		return fonts.DefaultFontStack
	case "icon-align":
		return constants.VAlignMiddle
	case "icon-height":
		return "32px"
	case "icon-position":
		return constants.AlignRight
	case "icon-unwrapped-alt":
		return "-"
	case "icon-unwrapped-url":
		return "https://i.imgur.com/w4uTygT.png"
	case "icon-width":
		return "32px"
	case "icon-wrapped-alt":
		return "+"
	case "icon-wrapped-url":
		return "https://i.imgur.com/bIXv1bk.png"
	case constants.MJMLPadding:
		return "10px 25px"
	default:
		return ""
	}
}

// MJAccordionTextComponent represents the mj-accordion-text component
type MJAccordionTextComponent struct {
	*BaseComponent
}

func NewMJAccordionTextComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJAccordionTextComponent {
	return &MJAccordionTextComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

// Render writes the text as mjml-accordion-text renders it, wherever it is:
// in its element, or misplaced, with a warning.
func (c *MJAccordionTextComponent) Render(w io.StringWriter) error {
	return renderAccordionText(w, c)
}

// writeContent writes the content as written: the parser keeps it raw, as
// MJML does for an ending tag.
func (c *MJAccordionTextComponent) writeContent(w io.StringWriter) error {
	if content := strings.TrimSpace(c.Node.Text); content != "" {
		if _, err := w.WriteString(content); err != nil {
			return err
		}
	}
	return nil
}

func (c *MJAccordionTextComponent) GetTagName() string {
	return "mj-accordion-text"
}

// explicitFontFamily returns the font-family attribute only when set directly
// on the mj-accordion-text element, without falling back to mj-class, global
// attributes, or component defaults, as resolveFontFamily reads rawAttrs.
func (c *MJAccordionTextComponent) explicitFontFamily() (string, bool) {
	return c.Node.LookupAttribute(constants.MJMLFontFamily)
}

func (c *MJAccordionTextComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLFontSize:
		return "13px"
	case constants.MJMLLineHeight:
		return "1"
	case constants.MJMLPadding:
		return "16px"
	case constants.MJMLFontFamily:
		return fonts.DefaultFontStack
	default:
		return ""
	}
}

// MJAccordionTitleComponent represents the mj-accordion-title component
type MJAccordionTitleComponent struct {
	*BaseComponent
}

func NewMJAccordionTitleComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJAccordionTitleComponent {
	return &MJAccordionTitleComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

// Render writes the title as mjml-accordion-title renders it, wherever it
// is: in its element, or misplaced, with a warning.
func (c *MJAccordionTitleComponent) Render(w io.StringWriter) error {
	return renderAccordionTitle(w, c)
}

// writeContent writes the content as written, as MJML does for an ending
// tag.
func (c *MJAccordionTitleComponent) writeContent(w io.StringWriter) error {
	content := strings.TrimSpace(c.Node.Text)
	if content != "" {
		if _, err := w.WriteString(content); err != nil {
			return err
		}
	}
	return nil
}

func (c *MJAccordionTitleComponent) GetTagName() string {
	return "mj-accordion-title"
}

// explicitFontFamily returns the font-family attribute only when set directly
// on the mj-accordion-title element, without falling back to mj-class,
// global attributes, or component defaults, as resolveFontFamily reads
// rawAttrs.
func (c *MJAccordionTitleComponent) explicitFontFamily() (string, bool) {
	return c.Node.LookupAttribute(constants.MJMLFontFamily)
}

func (c *MJAccordionTitleComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLFontSize:
		return "13px"
	case constants.MJMLPadding:
		return "16px"
	case constants.MJMLFontFamily:
		return fonts.DefaultFontStack
	default:
		return ""
	}
}

// MJAccordionElementComponent represents the mj-accordion-element component
type MJAccordionElementComponent struct {
	*BaseComponent
}

func NewMJAccordionElementComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJAccordionElementComponent {
	return &MJAccordionElementComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJAccordionElementComponent) Render(w io.StringWriter) error {
	// mjml-accordion-element writes a fixed 13px on its label, whatever the
	// font-size set on the element, mj-attributes or mj-all.
	const labelFontSize = "13px"
	fontFamily := c.resolveAttribute(c, constants.MJMLFontFamily)
	backgroundColor := c.resolveAttribute(c, constants.MJMLBackgroundColor)
	cssClass := c.resolveAttribute(c, constants.MJMLCSSClass)

	// The element's row, with its css-class and the mj-style inline rules
	// for it.
	row := html.NewHTMLTag("tr")
	if cssClass != "" {
		row.AddAttribute(constants.AttrClass, cssClass)
	}
	if err := row.RenderOpen(w); err != nil {
		return err
	}

	// Create TD with background color
	tdTag := html.NewHTMLTag("td").
		AddStyle(constants.CSSPadding, "0px")

	if backgroundColor != "" || c.hasAttribute(c, constants.MJMLBackgroundColor) {
		tdTag.AddStyle(constants.CSSBackgroundColor, backgroundColor)
	}

	if err := tdTag.RenderOpen(w); err != nil {
		return err
	}

	// Create label element
	labelTag := html.NewHTMLTag("label").
		AddAttribute(constants.AttrClass, "mj-accordion-element").
		AddStyle(constants.CSSFontSize, labelFontSize)

	if fontFamily != "" {
		labelTag.AddStyle(constants.CSSFontFamily, fontFamily)
	}

	if err := labelTag.RenderOpen(w); err != nil {
		return err
	}

	// Add checkbox input (hidden for accessibility)
	if _, err := w.WriteString("<!--[if !mso | IE]><!-->"); err != nil {
		return err
	}

	inputTag := html.NewHTMLTag("input").
		AddAttribute(constants.AttrClass, "mj-accordion-checkbox").
		AddAttribute(constants.AttrType, "checkbox").
		AddStyle(constants.CSSDisplay, constants.DisplayNone)

	if err := inputTag.RenderVoid(w); err != nil {
		return err
	}

	if _, err := w.WriteString("<!--<![endif]-->"); err != nil {
		return err
	}

	// Start accordion wrapper div
	if _, err := w.WriteString("<div>"); err != nil {
		return err
	}

	// As mjml-accordion-element's handleMissingChildren: a default title
	// first when there is none, every child in document order (titles, texts,
	// mj-raw, and a misplaced component, with a warning), then a default text
	// when there is none.
	noTitle, noText := c.missingTitleAndText()
	if noTitle {
		if err := c.defaultTitle().Render(w); err != nil {
			return err
		}
	}
	for _, child := range c.children {
		if err := child.Render(w); err != nil {
			return err
		}
	}
	if noText {
		if err := c.defaultText().Render(w); err != nil {
			return err
		}
	}

	// Close accordion wrapper div, label, cell, and row
	if _, err := w.WriteString("</div></label></td></tr>"); err != nil {
		return err
	}

	return nil
}

// missingTitleAndText reports whether the element has no mj-accordion-title
// and no mj-accordion-text, for which MJML renders a default one.
func (c *MJAccordionElementComponent) missingTitleAndText() (noTitle, noText bool) {
	noTitle, noText = true, true
	for _, child := range c.children {
		switch child.(type) {
		case *MJAccordionTitleComponent:
			noTitle = false
		case *MJAccordionTextComponent:
			noText = false
		}
	}
	return noTitle, noText
}

// defaultTitle and defaultText are the empty title and text MJML renders for
// an element that has none (handleMissingChildren). MJML builds them directly,
// with the attributes the element passes down but neither mj-attributes nor
// mj-all, so their options carry no global attributes.
// defaultTitle returns the title an element without one renders, with the
// attributes the element passes down, as handleMissingChildren creates it.
func (c *MJAccordionElementComponent) defaultTitle() *MJAccordionTitleComponent {
	title := NewMJAccordionTitleComponent(&parser.MJMLNode{XMLName: xml.Name{Local: "mj-accordion-title"}}, c.fallbackOpts())
	title.SetContext(ChildContextOf(c, 1, 0))
	return title
}

// defaultText returns the text an element without one renders, as
// defaultTitle.
func (c *MJAccordionElementComponent) defaultText() *MJAccordionTextComponent {
	text := NewMJAccordionTextComponent(&parser.MJMLNode{XMLName: xml.Name{Local: "mj-accordion-text"}}, c.fallbackOpts())
	text.SetContext(ChildContextOf(c, 1, 0))
	return text
}

func (c *MJAccordionElementComponent) fallbackOpts() *options.RenderOpts {
	if c.RenderOpts == nil {
		return nil
	}
	opts := *c.RenderOpts
	opts.GlobalAttributes = nil
	return &opts
}

// renderAccordionTitle renders an mj-accordion-title. As
// mjml-accordion-title, it reads its border and icon attributes as its own:
// its element passes them down (childContext), under the title's own values.
func renderAccordionTitle(w io.StringWriter, titleComponent *MJAccordionTitleComponent) error {
	border := titleComponent.resolveAttribute(titleComponent, constants.MJMLBorder)
	iconAlign := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconAlign)
	iconHeight := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconHeight)
	iconWidth := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconWidth)
	iconWrappedUrl := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconWrappedURL)
	iconUnwrappedUrl := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconUnwrappedURL)
	iconWrappedAlt := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconWrappedAlt)
	iconUnwrappedAlt := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconUnwrappedAlt)
	fontSize := titleComponent.resolveAttribute(titleComponent, constants.MJMLFontSize)
	explicitFontFamily, explicitFontFamilySet := titleComponent.explicitFontFamily()
	fontFamily, writeFontFamily := accordionChildFontFamily(titleComponent.Context(), explicitFontFamily, explicitFontFamilySet)
	padding := titleComponent.resolveAttribute(titleComponent, constants.MJMLPadding)
	paddingTop := titleComponent.resolveAttribute(titleComponent, constants.MJMLPaddingTop)
	paddingBottom := titleComponent.resolveAttribute(titleComponent, constants.MJMLPaddingBottom)
	paddingLeft := titleComponent.resolveAttribute(titleComponent, constants.MJMLPaddingLeft)
	paddingRight := titleComponent.resolveAttribute(titleComponent, constants.MJMLPaddingRight)

	// Get title-specific attributes
	backgroundColor := titleComponent.resolveAttribute(titleComponent, constants.MJMLBackgroundColor)
	color := titleComponent.resolveAttribute(titleComponent, constants.MJMLColor)
	cssClass := titleComponent.resolveAttribute(titleComponent, constants.MJMLCSSClass)

	// Get icon position to determine order
	iconPosition := titleComponent.resolveAttribute(titleComponent, constants.MJMLIconPosition)

	// Start title section
	divTag := html.NewHTMLTag("div").AddAttribute(constants.AttrClass, "mj-accordion-title")
	if err := divTag.RenderOpen(w); err != nil {
		return err
	}

	tableTag := html.NewHTMLTag("table").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddStyle(constants.CSSWidth, "100%")
	if border != "" { // none outside an accordion, as in MJML
		tableTag.AddStyle(constants.CSSBorderBottom, border)
	}

	if err := tableTag.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody><tr>"); err != nil {
		return err
	}

	// As mjml-accordion-title: the icon after the text only for "right",
	// before it otherwise (an empty icon-position included).
	if iconPosition != constants.AlignRight {
		if err := renderAccordionIconCell(w, iconAlign, iconHeight, iconWidth, iconWrappedUrl, iconUnwrappedUrl, iconWrappedAlt, iconUnwrappedAlt, titleComponent, backgroundColor); err != nil {
			return err
		}
	}

	// Create title cell
	tdTag := html.NewHTMLTag("td").
		AddStyle(constants.CSSWidth, "100%")

	// Add CSS class if specified
	if cssClass != "" {
		tdTag.AddAttribute(constants.AttrClass, cssClass)
	}

	// The background color first, when set, even empty (MJML order).
	if titleBackgroundSet(titleComponent, backgroundColor) {
		tdTag.AddStyle(constants.CSSBackgroundColor, backgroundColor)
	}

	// Add color if specified
	if color != "" {
		tdTag.AddStyle(constants.CSSColor, color)
	}

	// Add font size
	tdTag.AddStyle(constants.CSSFontSize, fontSize)

	if writeFontFamily {
		tdTag.AddStyle(constants.CSSFontFamily, fontFamily)
	}
	if titleComponent.hasAttribute(titleComponent, constants.MJMLFontWeight) {
		tdTag.AddStyle(constants.CSSFontWeight, titleComponent.resolveAttribute(titleComponent, constants.MJMLFontWeight))
	}

	// The padding, then its sides, in mjml-accordion's order: a side written
	// before the shorthand would be reset by it.
	tdTag.AddStyle(constants.CSSPadding, padding)
	if paddingBottom != "" {
		tdTag.AddStyle(constants.CSSPaddingBottom, paddingBottom)
	}
	if paddingLeft != "" {
		tdTag.AddStyle(constants.CSSPaddingLeft, paddingLeft)
	}
	if paddingRight != "" {
		tdTag.AddStyle(constants.CSSPaddingRight, paddingRight)
	}
	if paddingTop != "" {
		tdTag.AddStyle(constants.CSSPaddingTop, paddingTop)
	}

	if err := tdTag.RenderOpen(w); err != nil {
		return err
	}

	// Render title content
	if err := titleComponent.writeContent(w); err != nil {
		return err
	}

	// Close title cell
	if _, err := w.WriteString("</td>"); err != nil {
		return err
	}

	if iconPosition == constants.AlignRight {
		if err := renderAccordionIconCell(w, iconAlign, iconHeight, iconWidth, iconWrappedUrl, iconUnwrappedUrl, iconWrappedAlt, iconUnwrappedAlt, titleComponent, backgroundColor); err != nil {
			return err
		}
	}

	// Close table
	if _, err := w.WriteString("</tr></tbody></table></div>"); err != nil {
		return err
	}

	return nil
}

func renderAccordionIconCell(w io.StringWriter, iconAlign, iconHeight, iconWidth, iconWrappedUrl, iconUnwrappedUrl, iconWrappedAlt, iconUnwrappedAlt string, titleComponent *MJAccordionTitleComponent, titleBackgroundColor string) error {
	// Add icon cell using MSO conditional comments
	if _, err := w.WriteString("<!--[if !mso | IE]><!-->"); err != nil {
		return err
	}

	// Use titleComponent's default padding (16px) for icon, not accordion padding
	iconPadding := titleComponent.GetDefaultAttribute(constants.MJMLPadding)
	iconCellTag := html.NewHTMLTag("td").
		AddAttribute(constants.AttrClass, "mj-accordion-ico").
		AddStyle(constants.CSSPadding, iconPadding)

	// The title's background color, when set, even empty (MJML order -
	// before vertical-align).
	if titleBackgroundSet(titleComponent, titleBackgroundColor) {
		iconCellTag.AddStyle(constants.CSSBackground, titleBackgroundColor)
	}

	// Each icon attribute only when it is set, even empty, as mjml-core
	// leaves out undefined values: outside an accordion, which passes them
	// with its defaults, an element has none (vertical-align last).
	set := func(name string) bool {
		_, ok := attributeOf(titleComponent, name)
		return ok
	}
	if set(constants.MJMLIconAlign) {
		iconCellTag.AddStyle(constants.CSSVerticalAlign, iconAlign)
	}

	if err := iconCellTag.RenderOpen(w); err != nil {
		return err
	}

	// Wrapped state icon (plus)
	wrappedImg := html.NewHTMLTag("img")
	if set(constants.MJMLIconWrappedURL) {
		wrappedImg.AddAttribute(constants.AttrSrc, iconWrappedUrl)
	}
	if set(constants.MJMLIconWrappedAlt) {
		wrappedImg.AddAttribute(constants.AttrAlt, iconWrappedAlt)
	}
	wrappedImg.AddAttribute(constants.AttrClass, "mj-accordion-more").
		AddStyle(constants.CSSDisplay, constants.DisplayNone)
	if set(constants.MJMLIconWidth) {
		wrappedImg.AddStyle(constants.CSSWidth, iconWidth)
	}
	if set(constants.MJMLIconHeight) {
		wrappedImg.AddStyle(constants.CSSHeight, iconHeight)
	}

	if err := wrappedImg.RenderVoid(w); err != nil {
		return err
	}

	// Unwrapped state icon (minus)
	unwrappedImg := html.NewHTMLTag("img")
	if set(constants.MJMLIconUnwrappedURL) {
		unwrappedImg.AddAttribute(constants.AttrSrc, iconUnwrappedUrl)
	}
	if set(constants.MJMLIconUnwrappedAlt) {
		unwrappedImg.AddAttribute(constants.AttrAlt, iconUnwrappedAlt)
	}
	unwrappedImg.AddAttribute(constants.AttrClass, "mj-accordion-less").
		AddStyle(constants.CSSDisplay, constants.DisplayNone)
	if set(constants.MJMLIconWidth) {
		unwrappedImg.AddStyle(constants.CSSWidth, iconWidth)
	}
	if set(constants.MJMLIconHeight) {
		unwrappedImg.AddStyle(constants.CSSHeight, iconHeight)
	}

	if err := unwrappedImg.RenderVoid(w); err != nil {
		return err
	}

	// Close icon cell
	if _, err := w.WriteString("</td><!--<![endif]-->"); err != nil {
		return err
	}

	return nil
}

// renderAccordionText renders an mj-accordion-text, with its own border,
// which its element passes down (childContext), as mjml-accordion-text reads
// it.
func renderAccordionText(w io.StringWriter, textComponent *MJAccordionTextComponent) error {
	border := textComponent.resolveAttribute(textComponent, constants.MJMLBorder)
	fontSize := textComponent.resolveAttribute(textComponent, constants.MJMLFontSize)
	explicitFontFamily, explicitFontFamilySet := textComponent.explicitFontFamily()
	fontFamily, writeFontFamily := accordionChildFontFamily(textComponent.Context(), explicitFontFamily, explicitFontFamilySet)
	lineHeight := textComponent.resolveAttribute(textComponent, constants.MJMLLineHeight)
	padding := textComponent.resolveAttribute(textComponent, constants.MJMLPadding)
	paddingTop := textComponent.resolveAttribute(textComponent, constants.MJMLPaddingTop)
	paddingBottom := textComponent.resolveAttribute(textComponent, constants.MJMLPaddingBottom)
	paddingLeft := textComponent.resolveAttribute(textComponent, constants.MJMLPaddingLeft)
	paddingRight := textComponent.resolveAttribute(textComponent, constants.MJMLPaddingRight)

	// Get text-specific attributes
	backgroundColor := textComponent.resolveAttribute(textComponent, constants.MJMLBackgroundColor)
	color := textComponent.resolveAttribute(textComponent, constants.MJMLColor)
	cssClass := textComponent.resolveAttribute(textComponent, constants.MJMLCSSClass)

	// Start content section
	divTag := html.NewHTMLTag("div").AddAttribute(constants.AttrClass, "mj-accordion-content")
	if err := divTag.RenderOpen(w); err != nil {
		return err
	}

	tableTag := html.NewHTMLTag("table").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddStyle(constants.CSSWidth, "100%")
	if border != "" { // none outside an accordion, as in MJML
		tableTag.AddStyle(constants.CSSBorderBottom, border)
	}

	if err := tableTag.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody><tr>"); err != nil {
		return err
	}

	// Create content cell
	tdTag := html.NewHTMLTag("td")

	// Add CSS class if specified
	if cssClass != "" {
		tdTag.AddAttribute(constants.AttrClass, cssClass)
	}

	// The background color first, when set, even empty (MJML order).
	if backgroundColor != "" || textComponent.hasAttribute(textComponent, constants.MJMLBackgroundColor) {
		tdTag.AddStyle(constants.CSSBackground, backgroundColor)
	}

	// Add font size
	tdTag.AddStyle(constants.CSSFontSize, fontSize)

	// Add line height after font size
	tdTag.AddStyle(constants.CSSLineHeight, lineHeight)

	// Add color if specified
	if color != "" {
		tdTag.AddStyle(constants.CSSColor, color)
	}

	if writeFontFamily {
		tdTag.AddStyle(constants.CSSFontFamily, fontFamily)
	}
	for _, name := range [...]string{constants.MJMLFontWeight, constants.MJMLLetterSpacing} {
		if textComponent.hasAttribute(textComponent, name) {
			tdTag.AddStyle(name, textComponent.resolveAttribute(textComponent, name))
		}
	}

	// The padding, then its sides, in mjml-accordion's order: a side written
	// before the shorthand would be reset by it.
	tdTag.AddStyle(constants.CSSPadding, padding)
	if paddingBottom != "" {
		tdTag.AddStyle(constants.CSSPaddingBottom, paddingBottom)
	}
	if paddingLeft != "" {
		tdTag.AddStyle(constants.CSSPaddingLeft, paddingLeft)
	}
	if paddingRight != "" {
		tdTag.AddStyle(constants.CSSPaddingRight, paddingRight)
	}
	if paddingTop != "" {
		tdTag.AddStyle(constants.CSSPaddingTop, paddingTop)
	}

	if err := tdTag.RenderOpen(w); err != nil {
		return err
	}

	// Render text content
	if err := textComponent.writeContent(w); err != nil {
		return err
	}

	// Close content section
	if _, err := w.WriteString("</td></tr></tbody></table></div>"); err != nil {
		return err
	}

	return nil
}

func (c *MJAccordionElementComponent) GetTagName() string {
	return "mj-accordion-element"
}

// GetDefaultAttribute returns no default: mjml-accordion-element has none
// (its title.img defaults are not attributes). Inside an mj-accordion, the
// border and icon attributes come from the accordion, with its defaults,
// as passed attributes; an element outside any accordion has none, as in
// MJML.
func (c *MJAccordionElementComponent) GetDefaultAttribute(name string) string {
	return ""
}

// accordionChildrenAttributes are the attributes mj-accordion passes to its
// elements (childrenAttr in MJML's mj-accordion), with its own defaults.
var accordionChildrenAttributes = map[string]struct{}{
	constants.MJMLBorder:           {},
	constants.MJMLIconAlign:        {},
	constants.MJMLIconWidth:        {},
	constants.MJMLIconHeight:       {},
	constants.MJMLIconPosition:     {},
	constants.MJMLIconWrappedURL:   {},
	constants.MJMLIconWrappedAlt:   {},
	constants.MJMLIconUnwrappedURL: {},
	constants.MJMLIconUnwrappedAlt: {},
}

// titleBackgroundSet reports whether an accordion title writes its
// background color: when it resolves to a value, or when a layer sets it
// empty (mjml-core writes every style that is not undefined).
func titleBackgroundSet(title *MJAccordionTitleComponent, value string) bool {
	return value != "" || title.hasAttribute(title, constants.MJMLBackgroundColor)
}

// accordionChildFontFamily returns the font-family of an mj-accordion-title
// or mj-accordion-text cell, following resolveFontFamily in MJML 4.18: the
// font-family set on the child tag itself, otherwise the nearest element's,
// otherwise the nearest accordion's (which defaults to the Ubuntu stack), as
// their getChildContext pass them down to every descendant; none outside any
// accordion. Values coming from mj-attributes for the child are ignored, as
// in MJML. A font-family written empty on the child itself is its value:
// MJML then writes font-family:;. write reports whether the cell gets a
// font-family. It has no side effects, so the render-metadata pass can use
// it.
func accordionChildFontFamily(ctx ChildContext, explicit string, explicitSet bool) (value string, write bool) {
	switch {
	case explicitSet:
		return explicit, true
	case ctx.elementFontFamily != "":
		return ctx.elementFontFamily, true
	case ctx.accordionFontFamily != "":
		return ctx.accordionFontFamily, true
	}
	return "", false
}

// childContext passes the accordion's border and icon attributes down to its
// elements, as mj-accordion's renderChildren does.
func (c *MJAccordionComponent) childContext(ctx ChildContext) ChildContext {
	ctx.PassedAttributes = c.passedAttributes(c, accordionChildrenAttributes)
	// As mjml-accordion's getChildContext, for every descendant.
	ctx.accordionFontFamily = c.resolveAttribute(c, constants.MJMLFontFamily)
	return ctx
}

// childContext passes the element's border and icon attributes down to its
// title and text, as mj-accordion-element's renderChildren does.
func (c *MJAccordionElementComponent) childContext(ctx ChildContext) ChildContext {
	ctx.PassedAttributes = c.passedAttributes(c, accordionChildrenAttributes)
	// As mjml-accordion-element's getChildContext, for every descendant: the
	// element's own font-family (the accordion does not pass it down), even
	// none, which then replaces an outer element's.
	ctx.elementFontFamily = c.resolveAttribute(c, constants.MJMLFontFamily)
	return ctx
}
