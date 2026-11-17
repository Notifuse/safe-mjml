package components

import (
	"io"
	"math"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// Email layout constants: mj-body defines the default width, as mjml-body's
// defaultAttributes (width: '600px').
const (
	// DefaultBodyWidth is the default width of the email body in string format with units
	DefaultBodyWidth = "600px"

	// DefaultBodyWidthPixels is the default width of the email body as integer pixels
	DefaultBodyWidthPixels = 600
)

// MJBodyComponent represents mj-body
type MJBodyComponent struct {
	*BaseComponent

	// Preview is the last mj-preview of the head, which the body writes
	// right after <body>: each one replaces the previous one in
	// mjml-head-preview (add('preview')).
	Preview *MJPreviewComponent
}

// NewMJBodyComponent creates a new mj-body component
func NewMJBodyComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJBodyComponent {
	return &MJBodyComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJBodyComponent) GetTagName() string {
	return "mj-body"
}

// GetEffectiveWidth returns the body width in pixels, allowing the width
// attribute to override the default 600px container width. This ensures
// section children inherit the correct width.
func (c *MJBodyComponent) GetEffectiveWidth() int {
	if c.hasCtx {
		return lengthFromFloat(c.ctx.ContainerWidth)
	}
	if width, ok := c.pixelWidth(); ok {
		return max(lengthFromFloat(width), 0)
	}
	return GetDefaultBodyWidthPixels()
}

// pixelWidth returns the body's width attribute in pixels, as MJML reads the
// width mj-body passes as written to its blocks: parseFloat's number (600.5
// for 600.5px, 500 for 500PX), which the blocks that read it with parseInt
// truncate. A number with an exponent (1e2px) is parseInt's (1), as every
// block but mj-hero reads it; parseFloat and parseInt stop at what follows
// the digits (1 for 1_000px). It reports false when no layer sets one or it
// holds no number.
func (c *MJBodyComponent) pixelWidth() (float64, bool) {
	widthAttr := c.resolveAttribute(c, constants.MJMLWidth)
	if widthAttr == "" {
		return 0, false
	}
	width, ok := jsParseFloat(widthAttr)
	if !ok {
		return 0, false
	}
	if n, _ := jsParseIntValue(widthAttr); !math.IsInf(n, 0) && n != math.Trunc(width) {
		return n, true
	}
	return width, true
}

// Render writes <body>, the preview text and the email's <div>, as
// mjml-body's render does since MJML 5 (the skeleton no longer writes the
// body): the body gets the id and the css-class, both get word-spacing and
// the background color, set even to the empty string (by mj-all, say), as
// MJML writes background-color:;.
func (c *MJBodyComponent) Render(w io.StringWriter) error {
	backgroundColor, hasBackground := attributeOf(c, constants.MJMLBackgroundColor)
	addStyles := func(tag *html.HTMLTag) {
		tag.AddStyle(constants.CSSWordSpacing, constants.WordSpacingNormal)
		if hasBackground {
			tag.AddStyle(constants.CSSBackgroundColor, backgroundColor)
		}
	}

	bodyTag := html.NewHTMLTag("body")
	if id := c.resolveAttribute(c, constants.AttrID); id != "" {
		bodyTag.AddAttribute(constants.AttrID, id)
	}
	if classAttr := c.buildClassAttribute(""); classAttr != "" {
		bodyTag.AddAttribute(constants.AttrClass, classAttr)
	}
	addStyles(bodyTag)
	if err := bodyTag.RenderOpen(w); err != nil {
		return err
	}
	if c.Preview != nil {
		if err := c.Preview.Render(w); err != nil {
			return err
		}
	}

	bodyDiv := html.NewHTMLTag("div")
	if title := strings.TrimSpace(c.RenderOpts.Title); title != "" {
		bodyDiv.AddAttribute(constants.AttrAriaLabel, title)
	}
	bodyDiv.AddAttribute("aria-roledescription", "email").
		AddAttribute("role", "article")
	if langAttr := c.RenderOpts.Lang; langAttr != "" {
		dir := c.RenderOpts.Dir
		if dir == "" {
			dir = constants.DirAuto
		}
		bodyDiv.AddAttribute("lang", langAttr).
			AddAttribute("dir", dir)
	}
	addStyles(bodyDiv)
	if err := bodyDiv.RenderOpen(w); err != nil {
		return err
	}

	// Every block closes its own Outlook conditionals; adjacent ones are
	// merged over the whole document (html.OutlookMergeWriter).
	for _, child := range c.children {
		if isEmptyRaw(child) {
			continue
		}
		if err := child.Render(w); err != nil {
			return err
		}
	}

	_, err := w.WriteString("</div></body>")
	return err
}

func (c *MJBodyComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLWidth:
		return DefaultBodyWidth
	default:
		return ""
	}
}

// GetDefaultBodyWidthPixels returns the default body width as integer pixels
func GetDefaultBodyWidthPixels() int {
	return DefaultBodyWidthPixels
}

// layoutWidth returns the width mj-body gives its children: its width
// attribute as written, which can have a fraction (600.5px), or the default
// width.
// childContext gives the blocks of the body its width as written, as
// mjml-body passes getAttribute('width') as their containerWidth, when it is
// a length in pixels safe-mjml keeps (not negative, not saturated).
func (c *MJBodyComponent) childContext(ctx ChildContext) ChildContext {
	if width, ok := c.pixelWidth(); ok && width >= 0 && width <= maxLength {
		// As written, its unit included (max-width:500PX), as MJML writes
		// it in its sections.
		ctx.containerWidthText = c.resolveAttribute(c, constants.MJMLWidth)
	}
	return ctx
}

func (c *MJBodyComponent) layoutWidth() float64 {
	if c.hasCtx {
		return c.ctx.ContainerWidth
	}
	if width, ok := c.pixelWidth(); ok {
		// Saturated like every length (lengthFromFloat).
		return min(max(width, 0), maxLength)
	}
	return float64(GetDefaultBodyWidthPixels())
}
