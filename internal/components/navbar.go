package components

import (
	"io"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/fonts"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJNavbarComponent represents the mj-navbar component
type MJNavbarComponent struct {
	*BaseComponent
}

func NewMJNavbarComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJNavbarComponent {
	return &MJNavbarComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJNavbarComponent) Render(w io.StringWriter) error {
	// As mjml-navbar: the hamburger only for the value "hamburger".
	hamburger := c.resolveAttribute(c, constants.MJMLHamburger) == constants.MJMLHamburger

	// Render hamburger checkbox and trigger (mobile only). The checkbox ID is
	// only generated here, as MJML does in renderHamburger: a navbar without
	// hamburger writes no ID, so it must not take one from the generator.
	if hamburger {
		if err := c.renderHamburgerToggle(w, c.generateCheckboxID()); err != nil {
			return err
		}
	}

	// Render inline links container
	if err := c.renderInlineLinks(w); err != nil {
		return err
	}

	return nil
}

func (c *MJNavbarComponent) renderHamburgerToggle(w io.StringWriter, checkboxID string) error {
	// Render checkbox input (hidden)
	if _, err := w.WriteString("<!--[if !mso]><!-->"); err != nil {
		return err
	}

	inputTag := html.NewHTMLTag("input").
		AddAttribute(constants.AttrType, "checkbox").
		AddAttribute("id", checkboxID).
		AddAttribute(constants.AttrClass, "mj-menu-checkbox").
		AddAttribute(constants.AttrStyle, "display:none !important; max-height:0; visibility:hidden;")

	if err := inputTag.RenderVoid(w); err != nil {
		return err
	}

	if _, err := w.WriteString("<!--<![endif]-->"); err != nil {
		return err
	}

	// Render hamburger trigger/label
	triggerDiv := html.NewHTMLTag("div").
		AddAttribute(constants.AttrClass, "mj-menu-trigger").
		AddStyle(constants.CSSDisplay, constants.DisplayNone).
		AddStyle(constants.CSSMaxHeight, "0px").
		AddStyle(constants.CSSMaxWidth, "0px").
		AddStyle(constants.CSSFontSize, "0px").
		AddStyle(constants.CSSOverflow, "hidden")

	if err := triggerDiv.RenderOpen(w); err != nil {
		return err
	}

	// Render label with hamburger icons
	if err := c.renderHamburgerLabel(w, checkboxID); err != nil {
		return err
	}

	if _, err := w.WriteString("</div>"); err != nil {
		return err
	}

	return nil
}

func (c *MJNavbarComponent) renderHamburgerLabel(w io.StringWriter, checkboxID string) error {
	icoAlign := c.resolveAttribute(c, constants.MJMLIcoAlign)
	icoColor := c.resolveAttribute(c, constants.MJMLIcoColor)
	icoFontSize := c.resolveAttribute(c, constants.MJMLIcoFontSize)
	icoFontFamily := c.resolveAttribute(c, constants.MJMLIcoFontFamily)
	icoTextTransform := c.resolveAttribute(c, constants.MJMLIcoTextTransform)
	icoTextDecoration := c.resolveAttribute(c, constants.MJMLIcoTextDecoration)
	icoLineHeight := c.resolveAttribute(c, constants.MJMLIcoLineHeight)
	icoPadding := c.resolveAttribute(c, constants.MJMLIcoPadding)
	icoOpen := c.resolveAttribute(c, constants.MJMLIcoOpen)
	icoClose := c.resolveAttribute(c, constants.MJMLIcoClose)

	// Handle individual padding properties
	icoPaddingTop := c.resolveAttribute(c, constants.MJMLIcoPaddingTop)
	icoPaddingRight := c.resolveAttribute(c, constants.MJMLIcoPaddingRight)
	icoPaddingBottom := c.resolveAttribute(c, constants.MJMLIcoPaddingBottom)
	icoPaddingLeft := c.resolveAttribute(c, constants.MJMLIcoPaddingLeft)

	labelTag := html.NewHTMLTag("label").
		AddAttribute(constants.AttrAlign, icoAlign).
		AddAttribute(constants.AttrFor, checkboxID).
		AddAttribute(constants.AttrClass, "mj-menu-label").
		AddStyle(constants.CSSDisplay, constants.DisplayBlock).
		AddStyle(constants.CSSCursor, "pointer").
		AddStyle(constants.CSSMSOHide, "all").
		AddStyle(constants.CSSMozUserSelect, "none").
		AddStyle(constants.CSSUserSelect, "none").
		AddStyle(constants.CSSColor, icoColor).
		AddStyle(constants.CSSFontSize, icoFontSize).
		AddStyle(constants.CSSFontFamily, icoFontFamily).
		AddStyle(constants.CSSTextTransform, icoTextTransform).
		AddStyle(constants.CSSTextDecoration, icoTextDecoration).
		AddStyle(constants.CSSLineHeight, icoLineHeight).
		AddStyle(constants.CSSPadding, icoPadding)

	// The side paddings a layer sets, even empty, as mjml-core writes every
	// style that is not undefined.
	if c.hasAttribute(c, constants.MJMLIcoPaddingTop) {
		labelTag.AddStyle(constants.CSSPaddingTop, icoPaddingTop)
	}
	if c.hasAttribute(c, constants.MJMLIcoPaddingRight) {
		labelTag.AddStyle(constants.CSSPaddingRight, icoPaddingRight)
	}
	if c.hasAttribute(c, constants.MJMLIcoPaddingBottom) {
		labelTag.AddStyle(constants.CSSPaddingBottom, icoPaddingBottom)
	}
	if c.hasAttribute(c, constants.MJMLIcoPaddingLeft) {
		labelTag.AddStyle(constants.CSSPaddingLeft, icoPaddingLeft)
	}

	if err := labelTag.RenderOpen(w); err != nil {
		return err
	}

	// Render open icon (hamburger)
	openSpan := html.NewHTMLTag("span").
		AddAttribute(constants.AttrClass, "mj-menu-icon-open").
		AddStyle(constants.CSSMSOHide, "all")

	if err := openSpan.RenderOpen(w); err != nil {
		return err
	}
	// mjml-navbar writes the icon between line breaks; its minified output
	// keeps one space after it, before the close icon's span.
	if _, err := w.WriteString(icoOpen + " "); err != nil {
		return err
	}
	if _, err := w.WriteString("</span>"); err != nil {
		return err
	}

	// Render close icon (X)
	closeSpan := html.NewHTMLTag("span").
		AddAttribute(constants.AttrClass, "mj-menu-icon-close").
		AddStyle(constants.CSSDisplay, constants.DisplayNone).
		AddStyle(constants.CSSMSOHide, "all")

	if err := closeSpan.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString(icoClose); err != nil {
		return err
	}
	if _, err := w.WriteString("</span>"); err != nil {
		return err
	}

	if _, err := w.WriteString("</label>"); err != nil {
		return err
	}

	return nil
}

func (c *MJNavbarComponent) renderInlineLinks(w io.StringWriter) error {
	// Start inline links container
	linksDiv := html.NewHTMLTag("div").
		AddAttribute(constants.AttrClass, "mj-inline-links")

	if err := linksDiv.RenderOpen(w); err != nil {
		return err
	}

	// MSO table for Outlook compatibility with correct alignment
	align := c.resolveAttribute(c, constants.MJMLAlign)
	if _, err := w.WriteString("<!--[if mso | IE]><table border=\"0\" cellpadding=\"0\" cellspacing=\"0\" role=\"presentation\" align=\"" + html.EscapeAttribute(align) + "\"><tr><![endif]-->"); err != nil {
		return err
	}

	// As mjml-navbar's renderChildren: each link in its own Outlook cell,
	// any other child as it is; the final merge joins the adjacent
	// conditionals.
	afterRaw := false
	for _, child := range c.children {
		switch v := child.(type) {
		case *MJNavbarLinkComponent:
			// mjml-navbar writes each child between line breaks; once
			// minified, the ones after an mj-raw leave a space before the
			// next link, which separates them as displayed.
			if err := v.renderInCell(w, afterRaw); err != nil {
				return err
			}
			afterRaw = false
		default:
			// An mj-raw, or any other component (misplaced, with its
			// warning), as its own render writes it.
			if err := child.Render(w); err != nil {
				return err
			}
			afterRaw = !child.IsRawElement() || !isEmptyRaw(child)
		}
	}

	if _, err := w.WriteString("<!--[if mso | IE]></tr></table><![endif]-->"); err != nil {
		return err
	}

	if _, err := w.WriteString("</div>"); err != nil {
		return err
	}

	return nil
}

func (c *MJNavbarComponent) generateCheckboxID() string {
	return c.newElementID()
}

// rendersChild reports whether Render writes child: any component, in
// document order, as mjml-navbar's renderChildren.
func (c *MJNavbarComponent) rendersChild(child Component) bool {
	return true
}

func (c *MJNavbarComponent) GetTagName() string {
	return "mj-navbar"
}

func (c *MJNavbarComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLAlign:
		return constants.AlignCenter
	case "ico-align":
		return constants.AlignCenter
	case "ico-close":
		return "&#8855;"
	case "ico-color":
		return "#000000"
	case "ico-font-family":
		return fonts.DefaultFontStack
	case "ico-font-size":
		return "30px"
	case "ico-line-height":
		return "30px"
	case "ico-open":
		return "&#9776;"
	case "ico-padding":
		return "10px"
	case "ico-text-decoration":
		return constants.TextDecorationNone
	case "ico-text-transform":
		return "uppercase"
	default:
		return ""
	}
}

// MJNavbarLinkComponent represents the mj-navbar-link component
type MJNavbarLinkComponent struct {
	*BaseComponent
}

func NewMJNavbarLinkComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJNavbarLinkComponent {
	return &MJNavbarLinkComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

// Render writes the link as mjml-navbar-link's render, in its own Outlook
// cell, inside an mj-navbar or not (an mj-navbar-link directly in an
// mj-column is rendered, with a warning, as in MJML).
func (c *MJNavbarLinkComponent) Render(w io.StringWriter) error {
	return c.renderInCell(w, false)
}

// renderInCell writes the link in its Outlook cell, with a space before the
// link when space is set.
func (c *MJNavbarLinkComponent) renderInCell(w io.StringWriter, space bool) error {
	if err := c.renderMSOCellOpen(w); err != nil {
		return err
	}
	if space {
		if _, err := w.WriteString(" "); err != nil {
			return err
		}
	}
	if err := c.renderLink(w); err != nil {
		return err
	}
	return html.RenderMSOConditional(w, "</td>")
}

// renderMSOCellOpen writes the open tag of the link's Outlook cell.
func (c *MJNavbarLinkComponent) renderMSOCellOpen(w io.StringWriter) error {
	if _, err := w.WriteString("<!--[if mso | IE]><td"); err != nil {
		return err
	}

	style := "padding:" + c.resolveAttribute(c, constants.MJMLPadding) + ";"
	if c.hasAttribute(c, constants.MJMLPaddingBottom) {
		pb := c.resolveAttribute(c, constants.MJMLPaddingBottom)
		style += "padding-bottom:" + pb + ";"
	}
	if c.hasAttribute(c, constants.MJMLPaddingTop) {
		pt := c.resolveAttribute(c, constants.MJMLPaddingTop)
		style += "padding-top:" + pt + ";"
	}
	if c.hasAttribute(c, constants.MJMLPaddingLeft) {
		pl := c.resolveAttribute(c, constants.MJMLPaddingLeft)
		style += "padding-left:" + pl + ";"
	}
	if c.hasAttribute(c, constants.MJMLPaddingRight) {
		pr := c.resolveAttribute(c, constants.MJMLPaddingRight)
		style += "padding-right:" + pr + ";"
	}

	if _, err := w.WriteString(" style=\"" + html.EscapeAttribute(style) + "\""); err != nil {
		return err
	}

	msoClass := ""
	if originalClass := c.resolveAttribute(c, constants.MJMLCSSClass); originalClass != "" {
		msoClass = suffixCSSClasses(originalClass, "outlook")
	}

	if _, err := w.WriteString(" class=\"" + html.EscapeAttribute(msoClass) + "\" "); err != nil {
		return err
	}

	if _, err := w.WriteString("><![endif]-->"); err != nil {
		return err
	}

	return nil
}

// renderLink writes the link itself, mjml-navbar-link's renderContent.
func (c *MJNavbarLinkComponent) renderLink(w io.StringWriter) error {
	href := c.resolveAttribute(c, constants.MJMLHref)
	baseURL := c.resolveAttribute(c, navbarBaseURL)
	target := c.resolveAttribute(c, constants.MJMLTarget)
	color := c.resolveAttribute(c, constants.CSSColor)
	fontFamily := c.resolveAttribute(c, constants.MJMLFontFamily)
	fontSize := c.resolveAttribute(c, constants.MJMLFontSize)
	fontStyle := c.resolveAttribute(c, constants.MJMLFontStyle)
	fontWeight := c.resolveAttribute(c, constants.MJMLFontWeight)
	lineHeight := c.resolveAttribute(c, constants.MJMLLineHeight)
	textDecoration := c.resolveAttribute(c, constants.MJMLTextDecoration)
	textTransform := c.resolveAttribute(c, constants.MJMLTextTransform)
	padding := c.resolveAttribute(c, constants.MJMLPadding)

	// Handle individual padding properties
	paddingTop := c.resolveAttribute(c, constants.MJMLPaddingTop)
	paddingRight := c.resolveAttribute(c, constants.MJMLPaddingRight)
	paddingBottom := c.resolveAttribute(c, constants.MJMLPaddingBottom)
	paddingLeft := c.resolveAttribute(c, constants.MJMLPaddingLeft)

	// The navbar's base-url and the href as written, concatenated, as
	// mjml-navbar-link writes them: no slash added or removed
	// (https://a.example + y gives https://a.exampley). A link without href
	// gets the base-url alone, where MJML writes the base-url followed by
	// "undefined" (Pierre's decision: no JavaScript artifact, as for NaN).
	fullHref := href
	if baseURL != "" {
		fullHref = baseURL + href
	}

	// Build CSS class attribute
	cssClass := "mj-link"
	if additionalClass := c.resolveAttribute(c, constants.MJMLCSSClass); additionalClass != "" {
		cssClass = cssClass + " " + additionalClass
	}

	// A link without href and without base-url has no href, as MJML leaves
	// out an undefined attribute; an empty href is written empty.
	linkTag := html.NewHTMLTag("a")
	if baseURL != "" || c.hasAttribute(c, constants.MJMLHref) {
		linkTag.AddAttribute(constants.AttrHref, fullHref)
	}
	linkTag.AddAttribute(constants.AttrTarget, target).
		AddAttribute(constants.AttrClass, cssClass).
		AddStyle(constants.CSSDisplay, constants.DisplayInlineBlock).
		AddStyle(constants.CSSColor, color).
		AddStyle(constants.CSSFontFamily, fontFamily).
		AddStyle(constants.CSSFontSize, fontSize).
		AddStyle(constants.CSSFontWeight, fontWeight)
	if c.hasAttribute(c, constants.MJMLLetterSpacing) {
		linkTag.AddStyle(constants.CSSLetterSpacing, c.resolveAttribute(c, constants.MJMLLetterSpacing))
	}
	linkTag.AddStyle(constants.CSSLineHeight, lineHeight).
		AddStyle(constants.CSSTextDecoration, textDecoration).
		AddStyle(constants.CSSTextTransform, textTransform).
		AddStyle(constants.CSSPadding, padding)

	// rel and name as set, even empty: the minifier keeps rel="" and
	// name="".
	c.addSetAttribute(c, linkTag, constants.MJMLRel)
	c.addSetAttribute(c, linkTag, constants.MJMLName)

	// The side paddings a layer sets, even empty, as mjml-core writes every
	// style that is not undefined.
	if c.hasAttribute(c, constants.MJMLPaddingTop) {
		linkTag.AddStyle(constants.CSSPaddingTop, paddingTop)
	}
	if c.hasAttribute(c, constants.MJMLPaddingRight) {
		linkTag.AddStyle(constants.CSSPaddingRight, paddingRight)
	}
	if c.hasAttribute(c, constants.MJMLPaddingBottom) {
		linkTag.AddStyle(constants.CSSPaddingBottom, paddingBottom)
	}
	if c.hasAttribute(c, constants.MJMLPaddingLeft) {
		linkTag.AddStyle(constants.CSSPaddingLeft, paddingLeft)
	}

	// Only add font-style if it's not empty
	if fontStyle != "" {
		linkTag.AddStyle(constants.CSSFontStyle, fontStyle)
	}

	if err := linkTag.RenderOpen(w); err != nil {
		return err
	}

	// The link's content. mjml-navbar-link writes it between line breaks:
	// its minified output keeps one space after it, which separates the
	// links as displayed ("A B", not "AB").
	content := strings.TrimSpace(c.Node.Text)
	if content != "" {
		content += " "
	}
	if _, err := w.WriteString(content); err != nil {
		return err
	}

	if _, err := w.WriteString("</a>"); err != nil {
		return err
	}

	return nil
}

func (c *MJNavbarLinkComponent) GetTagName() string {
	return "mj-navbar-link"
}

func (c *MJNavbarLinkComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.CSSColor:
		return "#000000"
	case constants.MJMLFontFamily:
		return fonts.DefaultFontStack
	case constants.MJMLFontSize:
		return "13px"
	case constants.MJMLFontWeight:
		return constants.FontWeightNormal
	case constants.MJMLLineHeight:
		return "22px"
	case constants.MJMLPadding:
		return "15px 10px"
	case constants.MJMLTarget:
		return constants.TargetBlank
	case constants.MJMLTextDecoration:
		return constants.TextDecorationNone
	case "text-transform":
		return "uppercase"
	default:
		return ""
	}
}

// navbarBaseURL is the attribute through which mj-navbar passes its base-url
// down to its links, as mjml-navbar's renderChildren does.
const navbarBaseURL = "navbarBaseUrl"

// childContext passes the navbar's base-url down to its links. mjml-navbar
// passes it even when the navbar has none (as null), which hides any
// navbarBaseUrl from mj-all; an empty value does the same here.
func (c *MJNavbarComponent) childContext(ctx ChildContext) ChildContext {
	ctx.PassedAttributes = map[string]string{navbarBaseURL: c.resolveAttribute(c, constants.MJMLBaseURL)}
	return ctx
}
