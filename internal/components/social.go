package components

import (
	"fmt"
	"io"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/debug"
	"github.com/Notifuse/safe-mjml/internal/fonts"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// socialNetworkDefaults describes the default MJML metadata for a social network.
type socialNetworkDefaults struct {
	backgroundColor        string
	iconURL                string
	shareURLTemplate       string
	shareURLSkipSubstrings []string
}

// baseSocialNetworkDefaults matches MJML's default network definitions.
var baseSocialNetworkDefaults = map[string]socialNetworkDefaults{
	"facebook": {
		backgroundColor:        "#3b5998",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/facebook.png",
		shareURLTemplate:       "https://www.facebook.com/sharer/sharer.php?u=[[URL]]",
		shareURLSkipSubstrings: []string{"facebook.com/sharer"},
	},
	"twitter": {
		backgroundColor:        "#55acee",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/twitter.png",
		shareURLTemplate:       "https://twitter.com/intent/tweet?url=[[URL]]",
		shareURLSkipSubstrings: []string{"twitter.com/", "x.com/"},
	},
	"x": {
		backgroundColor:        "#000000",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/twitter-x.png",
		shareURLTemplate:       "https://twitter.com/intent/tweet?url=[[URL]]",
		shareURLSkipSubstrings: []string{"twitter.com/", "x.com/"},
	},
	"google": {
		backgroundColor:        "#dc4e41",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/google-plus.png",
		shareURLTemplate:       "https://plus.google.com/share?url=[[URL]]",
		shareURLSkipSubstrings: []string{"plus.google.com/share"},
	},
	"pinterest": {
		backgroundColor:        "#bd081c",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/pinterest.png",
		shareURLTemplate:       "https://pinterest.com/pin/create/button/?url=[[URL]]&media=&description=",
		shareURLSkipSubstrings: []string{"pinterest.com/pin/create/button"},
	},
	"linkedin": {
		backgroundColor:        "#0077b5",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/linkedin.png",
		shareURLTemplate:       "https://www.linkedin.com/shareArticle?mini=true&url=[[URL]]&title=&summary=&source=",
		shareURLSkipSubstrings: []string{"linkedin.com/shareArticle"},
	},
	"instagram": {
		backgroundColor: "#3f729b",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/instagram.png",
	},
	"web": {
		backgroundColor: "#4BADE9",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/web.png",
	},
	"snapchat": {
		backgroundColor: "#FFFA54",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/snapchat.png",
	},
	"youtube": {
		backgroundColor: "#EB3323",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/youtube.png",
	},
	"tumblr": {
		backgroundColor:        "#344356",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/tumblr.png",
		shareURLTemplate:       "https://www.tumblr.com/widgets/share/tool?canonicalUrl=[[URL]]",
		shareURLSkipSubstrings: []string{"tumblr.com/widgets/share"},
	},
	"github": {
		backgroundColor: "#000000",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/github.png",
	},
	"xing": {
		backgroundColor:        "#296366",
		iconURL:                "https://www.mailjet.com/images/theme/v1/icons/ico-social/xing.png",
		shareURLTemplate:       "https://www.xing.com/app/user?op=share&url=[[URL]]",
		shareURLSkipSubstrings: []string{"xing.com/app/user?op=share"},
	},
	"vimeo": {
		backgroundColor: "#53B4E7",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/vimeo.png",
	},
	"medium": {
		backgroundColor: "#000000",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/medium.png",
	},
	"soundcloud": {
		backgroundColor: "#EF7F31",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/soundcloud.png",
	},
	"dribbble": {
		backgroundColor: "#D95988",
		iconURL:         "https://www.mailjet.com/images/theme/v1/icons/ico-social/dribbble.png",
	},
}

const shareURLPlaceholder = "[[URL]]"

// getSocialNetworkDefaults resolves MJML defaults for a given social element name.
func getSocialNetworkDefaults(name string) (socialNetworkDefaults, bool) {
	if name == "" {
		return socialNetworkDefaults{}, false
	}

	if defaults, exists := baseSocialNetworkDefaults[name]; exists {
		return defaults, true
	}

	// As mjml-social-element's table: each network, and the same network
	// with -noshare, which links to the href itself. Any other name
	// (facebook-foo) is a network MJML does not know.
	if baseName, ok := strings.CutSuffix(name, "-noshare"); ok {
		if defaults, exists := baseSocialNetworkDefaults[baseName]; exists {
			// Copy the struct to avoid mutating the base definition.
			resolved := defaults
			if resolved.shareURLTemplate != "" {
				resolved.shareURLTemplate = shareURLPlaceholder
			}
			return resolved, true
		}
	}

	return socialNetworkDefaults{}, false
}

// Attributes that mj-social-element is allowed to inherit from its parent.
// socialPassedAttributes are the attributes mj-social passes down to its
// elements, as getSocialElementAttributes in mjml-social; it also passes its
// inner-padding as their padding.
var socialPassedAttributes = map[string]struct{}{
	constants.MJMLBorder:         {},
	constants.MJMLBorderRadius:   {},
	constants.MJMLColor:          {},
	constants.MJMLFontFamily:     {},
	constants.MJMLFontSize:       {},
	constants.MJMLFontWeight:     {},
	constants.MJMLFontStyle:      {},
	constants.MJMLIconSize:       {},
	constants.MJMLIconHeight:     {},
	constants.MJMLIconPadding:    {},
	constants.MJMLTextPadding:    {},
	constants.MJMLLineHeight:     {},
	constants.MJMLTextDecoration: {},
}

// MJSocialComponent represents mj-social
type MJSocialComponent struct {
	*BaseComponent
}

// NewMJSocialComponent creates a new mj-social component
func NewMJSocialComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJSocialComponent {
	return &MJSocialComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJSocialComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLAlign:
		return constants.AlignCenter
	case constants.MJMLBorderRadius:
		return "3px"
	case constants.MJMLColor:
		return "#333333"
	case constants.MJMLFontFamily:
		return fonts.DefaultFontStack
	case constants.MJMLFontSize:
		return "13px"
	case constants.MJMLIconSize:
		return "20px"
	case constants.MJMLLineHeight:
		return "22px"
	case constants.MJMLMode:
		return constants.ModeHorizontal
	case constants.MJMLPadding:
		return "10px 25px"
	case constants.MJMLTableLayout:
		return "auto"
	case constants.MJMLTextDecoration:
		return constants.TextDecorationNone
	default:
		return ""
	}
}

// Render implements optimized Writer-based rendering for MJSocialComponent
func (c *MJSocialComponent) Render(w io.StringWriter) error {
	align := c.resolveAttribute(c, constants.MJMLAlign)
	mode := c.resolveAttribute(c, constants.MJMLMode)

	// As mjml-social: horizontal only for "horizontal", vertical otherwise
	// (an empty mode included).
	if mode != constants.ModeHorizontal {
		// Vertical mode: single table with margin:0px
		table := html.NewHTMLTag("table").
			AddAttribute("border", "0").
			AddAttribute("cellpadding", "0").
			AddAttribute("cellspacing", "0").
			AddAttribute("role", "presentation").
			AddStyle(constants.CSSMargin, "0px")

		if err := table.RenderOpen(w); err != nil {
			return err
		}
		if _, err := w.WriteString("<tbody>"); err != nil {
			return err
		}
		if err := c.writeBlank(w); err != nil {
			return err
		}

		// Every child as its own render writes it, in document order:
		// the elements as table rows, an mj-raw as it is, and any other
		// component (misplaced, with its warning), as mjml-social's
		// renderVertical.
		for _, child := range c.children {
			if err := child.Render(w); err != nil {
				return err
			}
		}

		if _, err := w.WriteString("</tbody>"); err != nil {
			return err
		}
		if err := table.RenderClose(w); err != nil {
			return err
		}
	} else {
		// Horizontal mode (default): MSO conditional with inline tables
		// The align as resolved, even empty, as mjml-social writes it.
		msoAlign := align

		// As mjml-social's renderChildren: the Outlook row, each element in its
		// own cell and mj-raw as it is; the final merge joins the adjacent
		// conditionals.
		msoTable := fmt.Sprintf(
			"<!--[if mso | IE]><table align=\"%s\" border=\"0\" cellpadding=\"0\" cellspacing=\"0\" role=\"presentation\" ><tr><![endif]-->",
			html.EscapeAttribute(msoAlign),
		)
		if _, err := w.WriteString(msoTable); err != nil {
			return err
		}
		for _, child := range c.children {
			if child.IsRawElement() {
				if err := child.Render(w); err != nil {
					return err
				}
				continue
			}
			// An element, or any other component (misplaced, with its
			// warning), in its cell and inline table.
			if err := c.renderHorizontalElement(w, child, msoAlign); err != nil {
				return err
			}
		}
		if _, err := w.WriteString("<!--[if mso | IE]></tr></table><![endif]-->"); err != nil {
			return err
		}
	}

	return nil
}

// renderHorizontalElement writes a child that is not raw in horizontal mode
// as mjml-social's renderHorizontal does: its own Outlook cell and an inline
// table holding the element's row (or, misplaced, the component's render).
func (c *MJSocialComponent) renderHorizontalElement(w io.StringWriter, element Component, align string) error {
	if err := html.RenderMSOConditional(w, "<td>"); err != nil {
		return err
	}
	table := html.NewHTMLTag("table").
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrRole, "presentation").
		AddAttribute(constants.AttrAlign, align).
		AddStyle(constants.CSSFloat, "none").
		AddStyle(constants.CSSDisplay, "inline-table")
	if err := table.RenderOpen(w); err != nil {
		return err
	}
	if _, err := w.WriteString("<tbody>"); err != nil {
		return err
	}
	if err := element.Render(w); err != nil {
		return err
	}
	if _, err := w.WriteString("</tbody>"); err != nil {
		return err
	}
	if err := table.RenderClose(w); err != nil {
		return err
	}
	return html.RenderMSOConditional(w, "</td>")
}

func (c *MJSocialComponent) GetTagName() string {
	return "mj-social"
}

// rendersChild reports whether Render writes child: any component, in
// document order, as mjml-social's renderChildren.
func (c *MJSocialComponent) rendersChild(child Component) bool {
	return true
}

// MJSocialElementComponent represents mj-social-element
type MJSocialElementComponent struct {
	*BaseComponent
}

// NewMJSocialElementComponent creates a new mj-social-element component
func NewMJSocialElementComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJSocialElementComponent {
	return &MJSocialElementComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJSocialElementComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLAlign:
		return constants.AlignLeft
	case constants.MJMLAlt:
		return ""
	case constants.MJMLBorder:
		return "0"
	case constants.MJMLBorderRadius:
		return "3px"
	case constants.MJMLColor:
		return "#000"
	case constants.MJMLFontFamily:
		return fonts.DefaultFontStack
	case constants.MJMLFontSize:
		return "13px"
	case constants.MJMLHref:
		return ""
	case constants.MJMLIconPosition:
		return constants.AlignLeft
	case constants.MJMLLineHeight:
		return "1"
	case constants.MJMLName:
		return ""
	case constants.MJMLPadding:
		return "4px"
	case constants.MJMLTarget:
		return constants.TargetBlank
	case constants.MJMLTextDecoration:
		return constants.TextDecorationNone
	case constants.MJMLTextPadding:
		return "4px 4px 4px 0"
	case constants.MJMLVerticalAlign:
		return constants.VAlignMiddle
	default:
		return ""
	}
}

// socialAttribute resolves an mj-social-element attribute: as every
// attribute (resolveAttribute), the element, mj-class, mj-attributes, what
// mj-social passes down, mj-all, then the default; for src and
// background-color, the network's value when that is empty, set to "" or
// not set, as getSocialAttributes does (getAttribute(attr) || network[attr]).
func (c *MJSocialElementComponent) socialAttribute(name string) string {
	value := c.resolveAttribute(c, name)
	if value != "" || (name != constants.MJMLSrc && name != constants.MJMLBackgroundColor) {
		return value
	}
	defaults, ok := getSocialNetworkDefaults(c.resolveAttribute(c, constants.MJMLName))
	switch {
	case !ok:
		return value
	case name == constants.MJMLSrc:
		return defaults.iconURL
	default:
		return defaults.backgroundColor
	}
}

// Render implements optimized Writer-based rendering for MJSocialElementComponent
func (c *MJSocialElementComponent) Render(w io.StringWriter) error {
	iconSize := c.socialAttribute(constants.MJMLIconSize)
	iconHeight := c.socialAttribute(constants.MJMLIconHeight)
	if iconHeight == "" {
		iconHeight = iconSize // fallback to icon-size
	}
	src := c.socialAttribute(constants.MJMLSrc)
	href := c.socialAttribute(constants.MJMLHref)
	alt := c.socialAttribute(constants.MJMLAlt)

	// Handle special sharing URL generation for known platforms
	nameAttr := c.resolveAttribute(c, constants.MJMLName)
	if href != "" {
		if defaults, ok := getSocialNetworkDefaults(nameAttr); ok && defaults.shareURLTemplate != "" {
			// Unlike MJML, which wraps every href, an href that already points to
			// the network (a profile or a share URL) is kept: on purpose, see
			// docs/differences-from-mjml.md and mj-social-href-on-network.
			hrefLower := strings.ToLower(href)
			skipShare := false
			for _, pattern := range defaults.shareURLSkipSubstrings {
				if strings.Contains(hrefLower, pattern) {
					skipShare = true
					break
				}
			}

			if !skipShare {
				template := defaults.shareURLTemplate
				if strings.Contains(template, shareURLPlaceholder) {
					href = strings.ReplaceAll(template, shareURLPlaceholder, href)
				} else {
					href = template
				}
			}
		}
	}
	// Note: Only generate default URLs when href is explicitly provided (even if empty like "#")
	// Don't add default URLs when no href attribute exists - those are text-only social elements
	target := c.socialAttribute(constants.MJMLTarget)
	backgroundColor := c.socialAttribute(constants.MJMLBackgroundColor)
	border := c.socialAttribute(constants.MJMLBorder)
	borderRadius := c.socialAttribute(constants.MJMLBorderRadius)

	// An element MJML has no icon for (an unknown name and no src) renders
	// all the same, with an image without src, as mjml-social-element does.

	// The row of the element, as mjml-social-element renders it; mj-social
	// wraps it in its own table in horizontal mode.
	// Add CSS class to tr if specified on individual social element
	row := html.NewHTMLTag("tr")
	c.addDebugAttribute(row, "social-element")
	if cssClass := c.resolveAttribute(c, constants.MJMLCSSClass); cssClass != "" {
		row.AddAttribute(constants.AttrClass, cssClass)
	}
	if err := row.RenderOpen(w); err != nil {
		return err
	}

	// The icon cell and the text cell, in the order of icon-position:
	// mjml-social-element puts the icon first only for "left" (its
	// default), the text first for any other value, an empty one included.
	renderIcon := func() error {
		// Padding cell
		paddingTd := html.NewHTMLTag("td")

		// Use element's own padding first, then fall back to inherited inner-padding
		// mj-social passes its inner-padding down as the elements' padding: it
		// comes after the element's own padding (tag, mj-class, mj-attributes)
		// and before mj-all and the default, as in mjml-core.
		iconPadding := c.resolveAttribute(c, constants.MJMLPadding)

		// The padding, then its sides, in mjml-social-element's order.
		paddingTd.AddStyle(constants.CSSPadding, iconPadding)
		c.addPaddingLonghands(c, paddingTd, sideTop, sideRight, sideBottom, sideLeft)

		paddingTd.AddStyle(constants.CSSVerticalAlign, c.socialAttribute(constants.MJMLVerticalAlign))

		if err := paddingTd.RenderOpen(w); err != nil {
			return err
		}

		// Inner table with background color
		innerTable := html.NewHTMLTag("table").
			AddAttribute("border", "0").
			AddAttribute("cellpadding", "0").
			AddAttribute("cellspacing", "0").
			AddAttribute("role", "presentation")
		// background-color || the network's, as mjml-social-element: none
		// for a network MJML has no icon for.
		if backgroundColor != "" {
			innerTable.AddStyle(constants.CSSBackground, backgroundColor)
		}
		innerTable.AddStyle(constants.CSSBorderRadius, borderRadius)
		// An empty icon-size falls back to the network's, which none defines:
		// MJML then writes no width and no height.
		if iconSize != "" {
			innerTable.AddStyle(constants.CSSWidth, iconSize)
		}

		if err := innerTable.RenderOpen(w); err != nil {
			return err
		}
		if _, err := w.WriteString("<tbody><tr>"); err != nil {
			return err
		}

		// Icon cell
		iconTd := html.NewHTMLTag("td")

		// Handle icon-padding if specified (check both element and inherited from parent)
		iconPaddingAttr := c.socialAttribute(constants.MJMLIconPadding)
		if iconPaddingAttr != "" {
			iconTd.AddStyle(constants.CSSPadding, iconPaddingAttr)
		}

		iconTd.AddStyle(constants.CSSFontSize, "0")
		if iconHeight != "" {
			iconTd.AddStyle(constants.CSSHeight, iconHeight)
		}
		iconTd.AddStyle(constants.CSSVerticalAlign, "middle")
		if iconSize != "" {
			iconTd.AddStyle(constants.CSSWidth, iconSize)
		}

		if err := iconTd.RenderOpen(w); err != nil {
			return err
		}

		// Image with optional link. MJML sets only its width attribute,
		// parseInt(icon-size) ("45%" gives 45); the height comes from the icon
		// cell.
		widthAttr := parseIntAttribute(iconSize)

		img := html.NewHTMLTag("img")
		img.AddAttribute(constants.AttrAlt, alt)
		// src || the network's icon: none for a network MJML has no icon for.
		if src != "" {
			img.AddAttribute(constants.AttrSrc, src)
		}
		// No icon-size (an element outside mj-social, or icon-size=""):
		// MJML writes width="NaN", safe-mjml no width.
		if widthAttr != "" {
			img.AddAttribute(constants.AttrWidth, widthAttr)
		}

		// Add title attribute if specified
		title := c.resolveAttribute(c, constants.MJMLTitle)
		if title != "" {
			img.AddAttribute("title", title)
		}

		// sizes and srcset as mjml-social's getSocialAttributes reads them: the
		// attribute when it has a value (no network defines them).
		for _, name := range [...]string{constants.AttrSizes, constants.AttrSrcset} {
			if value := c.resolveAttribute(c, name); value != "" {
				img.AddAttribute(name, value)
			}
		}

		img.AddStyle(constants.CSSBorder, border).
			AddStyle(constants.CSSBorderRadius, borderRadius).
			AddStyle(constants.CSSDisplay, "block")

		if href != "" {
			link := html.NewHTMLTag("a").
				AddAttribute("href", href)
			c.addRelAttribute(link)
			link.AddAttribute("target", target)
			if err := link.RenderOpen(w); err != nil {
				return err
			}
			if err := c.renderVoid(w, img); err != nil {
				return err
			}
			if err := link.RenderClose(w); err != nil {
				return err
			}
		} else {
			if err := c.renderVoid(w, img); err != nil {
				return err
			}
		}

		if err := iconTd.RenderClose(w); err != nil {
			return err
		}
		if _, err := w.WriteString("</tr></tbody>"); err != nil {
			return err
		}
		if err := innerTable.RenderClose(w); err != nil {
			return err
		}
		if err := paddingTd.RenderClose(w); err != nil {
			return err
		}
		return nil
	}
	renderContent := func() error {
		// Render text content if present - INSIDE the same <tr>
		// Use GetMixedContent to preserve HTML tags like <b>, <i>, etc. within text
		textContent := c.Node.GetMixedContent()
		if debug.Enabled() {
			debug.DebugLogWithData(
				"social-element",
				"content-selection",
				"Selected text content source",
				map[string]any{
					"element_name":   c.resolveAttribute(c, constants.MJMLName),
					"plain_text":     c.Node.Text,
					"mixed_content":  textContent,
					"has_children":   len(c.Node.Children) > 0,
					"content_length": len(textContent),
				},
			)
		}
		if textContent != "" {
			// Text cell with padding and styling
			// MJML's tdText is always middle-aligned; vertical-align goes on the
			// icon cell.
			textTd := html.NewHTMLTag("td").
				AddStyle(constants.CSSVerticalAlign, constants.VAlignMiddle).
				AddStyle(constants.CSSPadding, c.socialAttribute(constants.MJMLTextPadding)).
				AddStyle(constants.CSSTextAlign, c.socialAttribute(constants.MJMLAlign))

			if err := textTd.RenderOpen(w); err != nil {
				return err
			}

			// Text content with social styling - use <a> if href present, <span> otherwise
			var textElement *html.HTMLTag
			if href != "" {
				// Use <a> tag when there's a link
				textElement = html.NewHTMLTag("a").
					AddAttribute("href", href)
				c.addRelAttribute(textElement)
				textElement.AddAttribute("target", target)
			} else {
				// Use <span> tag when no link
				textElement = html.NewHTMLTag("span")
			}

			// Add styling - maintain MJML CSS property order
			textElement.AddStyle(constants.CSSColor, c.socialAttribute(constants.MJMLColor)).
				AddStyle(constants.CSSFontSize, c.socialAttribute(constants.MJMLFontSize))

			// font-weight and font-style as a layer or the mj-social sets them,
			// "normal" included: mjml-social-element has no default for either.
			if fontWeight := c.socialAttribute(constants.MJMLFontWeight); fontWeight != "" {
				textElement.AddStyle(constants.CSSFontWeight, fontWeight)
			}
			if fontStyle := c.socialAttribute(constants.MJMLFontStyle); fontStyle != "" {
				textElement.AddStyle(constants.CSSFontStyle, fontStyle)
			}

			textElement.AddStyle(constants.CSSFontFamily, c.socialAttribute(constants.MJMLFontFamily)).
				AddStyle(constants.CSSLineHeight, c.socialAttribute(constants.MJMLLineHeight)).
				AddStyle(constants.CSSTextDecoration, c.socialAttribute(constants.MJMLTextDecoration))

			if err := textElement.RenderOpen(w); err != nil {
				return err
			}
			if _, err := w.WriteString(textContent); err != nil {
				return err
			}
			if err := c.writeBlank(w); err != nil {
				return err
			}
			if err := textElement.RenderClose(w); err != nil {
				return err
			}
			if err := textTd.RenderClose(w); err != nil {
				return err
			}
		}
		return nil
	}
	first, second := renderIcon, renderContent
	if c.resolveAttribute(c, constants.MJMLIconPosition) != constants.AlignLeft {
		first, second = renderContent, renderIcon
	}
	if err := first(); err != nil {
		return err
	}
	if err := second(); err != nil {
		return err
	}

	return row.RenderClose(w)
}

func (c *MJSocialElementComponent) GetTagName() string {
	return "mj-social-element"
}

// childContext passes mj-social's text and icon attributes down to its
// elements, and its inner-padding as their padding, as mj-social's
// renderChildren does.
func (c *MJSocialComponent) childContext(ctx ChildContext) ChildContext {
	passed := c.passedAttributes(c, socialPassedAttributes)
	if innerPadding := c.resolveAttribute(c, constants.MJMLInnerPadding); innerPadding != "" {
		passed[constants.MJMLPadding] = innerPadding
	}
	ctx.PassedAttributes = passed
	return ctx
}

// addRelAttribute writes the rel of the element's links, as
// mjml-social-element does.
func (c *MJSocialElementComponent) addRelAttribute(link *html.HTMLTag) {
	c.addSetAttribute(c, link, constants.MJMLRel)
}
