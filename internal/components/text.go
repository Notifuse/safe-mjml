package components

import (
	"io"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/debug"
	"github.com/Notifuse/safe-mjml/internal/fonts"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJTextComponent represents mj-text
type MJTextComponent struct {
	*BaseComponent
}

// NewMJTextComponent creates a new mj-text component
func NewMJTextComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJTextComponent {
	return &MJTextComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJTextComponent) GetTagName() string {
	return "mj-text"
}

// Render implements optimized Writer-based rendering for MJTextComponent
func (c *MJTextComponent) Render(w io.StringWriter) error {
	if debug.Enabled() {
		debug.DebugLog("mj-text", "render-start", "Starting text component rendering")
		debug.DebugLogWithData("mj-text", "content", "Processing text content", map[string]any{
			"container_width": c.GetEffectiveWidth(),
		})
	}

	// Create inner div with font styling
	divTag := html.NewHTMLTag("div")
	c.addDebugAttribute(divTag, "text")

	// Apply font styles using the proper interface method
	fontFamily := c.resolveAttribute(c, constants.MJMLFontFamily)
	fontSize := c.resolveAttribute(c, constants.MJMLFontSize)
	fontWeight := c.resolveAttribute(c, constants.MJMLFontWeight)
	fontStyle := c.resolveAttribute(c, constants.MJMLFontStyle)
	color := c.resolveAttribute(c, constants.MJMLColor)
	lineHeight := c.resolveAttribute(c, constants.MJMLLineHeight)
	textAlign := c.resolveAttribute(c, constants.MJMLAlign)
	textDecoration := c.resolveAttribute(c, constants.MJMLTextDecoration)
	textTransform := c.resolveAttribute(c, constants.MJMLTextTransform)
	letterSpacing := c.resolveAttribute(c, constants.MJMLLetterSpacing)

	// Apply styles in the order MJML writes them
	if fontFamily != "" {
		divTag.AddStyle(constants.CSSFontFamily, fontFamily)
	}
	if fontSize != "" {
		divTag.AddStyle(constants.CSSFontSize, fontSize)
	}
	if fontWeight != "" {
		divTag.AddStyle(constants.CSSFontWeight, fontWeight)
	}
	if letterSpacing != "" {
		divTag.AddStyle(constants.CSSLetterSpacing, letterSpacing)
	}
	if lineHeight != "" {
		divTag.AddStyle(constants.CSSLineHeight, lineHeight)
	}
	if textAlign != "" {
		divTag.AddStyle(constants.CSSTextAlign, textAlign)
	}
	if textTransform != "" {
		divTag.AddStyle(constants.CSSTextTransform, textTransform)
	}
	if color != "" {
		divTag.AddStyle(constants.CSSColor, color)
	}
	if fontStyle != "" {
		divTag.AddStyle(constants.CSSFontStyle, fontStyle)
	}
	if textDecoration != "" {
		divTag.AddStyle(constants.CSSTextDecoration, textDecoration)
	}
	height := c.resolveAttribute(c, constants.MJMLHeight)
	if height != "" {
		divTag.AddStyle(constants.CSSHeight, height)
		// Outlook ignores the div height, so MJML wraps the div in a
		// fixed-height table for it.
		if err := renderMSOTextHeightOpen(w, height); err != nil {
			return err
		}
	}

	if err := divTag.RenderOpen(w); err != nil {
		return err
	}
	innerHTML, err := c.buildRawInnerHTML()
	if err != nil {
		return err
	}
	if innerHTML != "" {
		if _, err := w.WriteString(innerHTML); err != nil {
			return err
		}
	}
	if err := divTag.RenderClose(w); err != nil {
		return err
	}
	if height != "" {
		if err := html.RenderMSOConditional(w, "</td></tr></table>"); err != nil {
			return err
		}
	}

	return nil
}

// renderMSOTextHeightOpen opens the Outlook-only table MJML puts around an
// mj-text that has a height:
// <table role="presentation" border="0" cellpadding="0" cellspacing="0"><tr><td height="40px" style="vertical-align:top;height:40px;">
func renderMSOTextHeightOpen(w io.StringWriter, height string) error {
	var open strings.Builder
	table := html.NewHTMLTag("table").
		AddAttribute(constants.AttrRole, "presentation").
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0")
	if err := table.RenderOpen(&open); err != nil {
		return err
	}
	open.WriteString("<tr>")
	td := html.NewHTMLTag("td").
		AddAttribute(constants.AttrHeight, height).
		AddStyle(constants.CSSVerticalAlign, constants.VAlignTop).
		AddStyle(constants.CSSHeight, height)
	if err := td.RenderOpen(&open); err != nil {
		return err
	}
	return html.RenderMSOConditional(w, open.String())
}

func (c *MJTextComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLFontSize:
		return "13px"
	case constants.MJMLColor:
		return "#000000"
	case constants.MJMLAlign:
		return constants.AlignLeft
	case constants.MJMLFontFamily:
		return fonts.DefaultFontStack
	case constants.MJMLLineHeight:
		return "1"
	case constants.MJMLPadding:
		return "10px 25px"
	default:
		return ""
	}
}

// buildRawInnerHTML returns the content as written, trimmed, as mjml-core's
// getContent gives it: the parser keeps it raw, as MJML does for an ending
// tag. Its whitespace is not collapsed: MJML's render does not do it (only
// the minifier of the references does, which the comparison allows for),
// and collapsing it changed what <pre>, <textarea> and attribute values
// hold.
func (c *MJTextComponent) buildRawInnerHTML() (string, error) {
	// A no-break space stays the character it is, as in MJML: written as
	// &#xA0;, it no longer separated two classes for the inline mj-style.
	return strings.TrimSpace(c.Node.Text), nil
}
