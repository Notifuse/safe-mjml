package components

import (
	"io"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJDividerComponent represents mj-divider
type MJDividerComponent struct {
	*BaseComponent
}

// NewMJDividerComponent creates a new mj-divider component
func NewMJDividerComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJDividerComponent {
	return &MJDividerComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJDividerComponent) GetDefaultAttribute(name string) string {
	switch name {
	case "align":
		return "center"
	case "border-color":
		return "#000000"
	case "border-style":
		return "solid"
	case "border-width":
		return "4px"
	case "padding":
		return "10px 25px"
	case "width":
		return "100%"
	default:
		return ""
	}
}

// Render implements optimized Writer-based rendering for MJDividerComponent
func (c *MJDividerComponent) Render(w io.StringWriter) error {
	borderColor := c.resolveAttribute(c, constants.MJMLBorderColor)
	borderStyle := c.resolveAttribute(c, constants.MJMLBorderStyle)
	borderWidth := c.resolveAttribute(c, constants.MJMLBorderWidth)
	// As mjml-divider: the value as resolved, even empty, on the Outlook
	// table, and the margin of left, right or anything else.
	align := c.resolveAttribute(c, constants.MJMLAlign)

	margin := c.marginForAlign(align)

	// Create paragraph with border styles in MJML order
	p := html.NewHTMLTag("p")
	c.addDebugAttribute(p, "divider")
	p.
		AddStyle(constants.CSSBorderTop, borderStyle+" "+borderWidth+" "+borderColor).
		AddStyle(constants.CSSFontSize, "1px").
		AddStyle(constants.CSSMargin, margin)

	// Add width (MJML includes default width of 100%)
	width := c.resolveAttribute(c, constants.MJMLWidth)
	p = p.AddStyle(constants.CSSWidth, width)

	// Render paragraph - must be empty, not self-closing as MJML writes it
	if err := p.RenderOpen(w); err != nil {
		return err
	}
	if err := p.RenderClose(w); err != nil {
		return err
	}

	msoWidth := c.outlookWidth(width)

	// Build MSO table directly to writer to avoid fmt.Sprintf allocation
	if _, err := w.WriteString(`<!--[if mso | IE]><table align="`); err != nil {
		return err
	}
	if _, err := w.WriteString(html.EscapeAttribute(align)); err != nil {
		return err
	}
	if _, err := w.WriteString(`" border="0" cellpadding="0" cellspacing="0" style="border-top:`); err != nil {
		return err
	}
	if _, err := w.WriteString(html.EscapeAttribute(borderStyle)); err != nil {
		return err
	}
	if _, err := w.WriteString(" "); err != nil {
		return err
	}
	if _, err := w.WriteString(html.EscapeAttribute(borderWidth)); err != nil {
		return err
	}
	if _, err := w.WriteString(" "); err != nil {
		return err
	}
	if _, err := w.WriteString(html.EscapeAttribute(borderColor)); err != nil {
		return err
	}
	if _, err := w.WriteString(`;font-size:1px;margin:`); err != nil {
		return err
	}
	if _, err := w.WriteString(html.EscapeAttribute(margin)); err != nil {
		return err
	}
	if _, err := w.WriteString(`;width:`); err != nil {
		return err
	}
	if _, err := w.WriteString(msoWidth); err != nil {
		return err
	}
	if _, err := w.WriteString(`;" role="presentation" width="`); err != nil {
		return err
	}
	if _, err := w.WriteString(msoWidth); err != nil {
		return err
	}
	if _, err := w.WriteString(`" ><tr><td style="height:0;line-height:0;"> &nbsp; </td></tr></table><![endif]-->`); err != nil {
		return err
	}

	return nil
}

func (c *MJDividerComponent) GetTagName() string {
	return "mj-divider"
}

// outlookWidth returns the width of the divider's Outlook table, as
// mjml-divider's getOutlookWidth: a percentage of the container minus the
// left and right padding (the percentage read twice with parseInt, the
// product written as JavaScript prints it), a width in pixels or without
// unit as written (an empty width too), and otherwise the container minus
// the padding, as for a percentage that holds no number.
func (c *MJDividerComponent) outlookWidth(width string) string {
	// In float64, where the sum and difference of saturated lengths cannot
	// overflow, even with a 32-bit int.
	paddingSize := float64(c.shorthandAttrValue(c, constants.MJMLPadding, sideLeft)) +
		float64(c.shorthandAttrValue(c, constants.MJMLPadding, sideRight))
	// Never negative: a padding wider than the container leaves no room.
	effectiveWidth := max(float64(c.GetEffectiveWidth())-paddingSize, 0)
	switch widthUnit(width) {
	case "%":
		// widthParser reads the percentage with parseInt, and
		// getOutlookWidth reads that number again with parseInt, as
		// JavaScript writes it: 1e+21 is 1.
		parsed, ok := jsParseIntValue(width)
		if ok {
			parsed, ok = jsParseIntValue(jsNumber(parsed))
		}
		if !ok {
			// No number ("%", or more than 308 digits, which is Infinity
			// read again as NaN): MJML writes NaNpx, which Outlook ignores,
			// shrinking the line to nothing. safe-mjml writes no NaN and
			// gives the line the width of a 100% divider (Pierre's
			// decision, 2026-11-18).
			return jsPixels(effectiveWidth)
		}
		return jsPixels(effectiveWidth * (parsed / 100))
	case "px":
		return width
	default:
		return jsPixels(effectiveWidth)
	}
}

func (c *MJDividerComponent) marginForAlign(align string) string {
	switch align {
	case constants.AlignLeft:
		return "0px"
	case constants.AlignRight:
		return "0px 0px 0px auto"
	default:
		return "0px auto"
	}
}
