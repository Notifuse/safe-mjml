package components

import (
	"io"
	"math"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
	"github.com/Notifuse/safe-mjml/internal/styles"
)

// MJGroupComponent represents mj-group - horizontal grouping of columns
type MJGroupComponent struct {
	*BaseComponent
}

// NewMJGroupComponent creates a new mj-group component
func NewMJGroupComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJGroupComponent {
	return &MJGroupComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJGroupComponent) GetDefaultAttribute(name string) string {
	switch name {
	case "direction":
		return "ltr"
	default:
		return ""
	}
}

func (c *MJGroupComponent) GetTagName() string {
	return "mj-group"
}

// GetParsedWidth returns the width the group takes in its section, following
// mjml-group's getParsedWidth: the resolved width attribute (element, mj-class,
// mj-attributes), otherwise an equal share of the non-raw siblings. Pixel
// widths are truncated to an integer like MJML's parseInt.
func (c *MJGroupComponent) GetParsedWidth() styles.Size {
	if width, ok := c.parsedWidthAttribute(); ok {
		return width.size
	}
	nonRawSiblings := c.getNonRawSiblings()
	if nonRawSiblings <= 0 {
		nonRawSiblings = 1
	}
	return styles.NewPercentSize(100.0 / float64(nonRawSiblings))
}

// parsedWidthAttribute returns the group's width attribute as mjml-core's
// widthParser reads it (any unit but "%" as pixels), or false when no layer
// sets one or it holds no number.
func (c *MJGroupComponent) parsedWidthAttribute() (mjmlWidth, bool) {
	width := c.resolveAttribute(c, constants.MJMLWidth)
	if width == "" {
		return mjmlWidth{}, false
	}
	return parseMJMLWidth(width)
}

// GetColumnClass returns the responsive class of the group and the width its
// media-query rule must use. Unlike mj-column, MJML names the group class after
// parseInt of the width (33.5% -> mj-column-per-33), while the rule
// keeps the exact width, with its unit as written (80%;).
func (c *MJGroupComponent) GetColumnClass() (string, string) {
	width, ok := c.parsedWidthAttribute()
	if !ok {
		width = mjmlWidth{size: c.GetParsedWidth(), unit: "%"}
	}
	// parseInt(parsedWidth, 10), which reads the number as JavaScript
	// writes it: 999999999999 stays whole, 0.0000001 is "1e-7", so 1.
	integerPart := jsParseInt(jsNumber(width.size.Value()))
	if width.size.IsPercent() {
		return "mj-column-per-" + integerPart, width.cssText()
	}
	return "mj-column-px-" + integerPart, width.cssText()
}

// exactWidth returns the group's width in its container with its fraction,
// as mjml-group's getWidthAsPixel computes it (parseFloat of the container
// width).
func (c *MJGroupComponent) exactWidth() float64 {
	return widthIn(c.GetParsedWidth(), c.BaseComponent.layoutWidth())
}

// GetWidthAsPixel returns the group width in pixels for its Outlook cell.
func (c *MJGroupComponent) GetWidthAsPixel() string {
	// Not rounded, as mjml-group writes it.
	return jsPixels(c.exactWidth())
}

// outlookVerticalAlign returns the vertical-align of the Outlook cell a
// section opens for this group, when a layer sets it, even empty (mjml-group
// has no default for it, and mjml-core writes every style that is not
// undefined).
func (c *MJGroupComponent) outlookVerticalAlign() (string, bool) {
	return attributeOf(c, constants.MJMLVerticalAlign)
}

// outlookCellWidth returns the width of the Outlook cell mjml-group writes
// around a child that is not a column, as its getElementWidth computes it:
// the child's own width for a group (getWidthAsPixel), else its width
// attribute; without one, the group's container width (parseInt) shared
// among the group's siblings. A percentage is divided by the group's child
// width, a "123px" string, which gives NaN in MJML: safe-mjml then writes no
// width (it writes no NaN, Pierre's rule), and reports false.
func (c *MJGroupComponent) outlookCellWidth(child Component) (string, bool) {
	var width string
	if group, ok := child.(*MJGroupComponent); ok {
		width = group.GetWidthAsPixel()
	} else {
		width, _ = attributeOf(child, constants.MJMLWidth)
	}
	if width == "" {
		share := math.Trunc(c.Context().ContainerWidth) / float64(max(c.getNonRawSiblings(), 1))
		return jsPixels(share), true
	}
	parsed, ok := parseMJMLWidth(width)
	if !ok || parsed.size.IsPercent() {
		return "", false
	}
	return parsed.cssText(), true
}

// Render implements optimized Writer-based rendering for MJGroupComponent
func (c *MJGroupComponent) Render(w io.StringWriter) error {
	direction := c.resolveAttribute(c, constants.MJMLDirection)
	verticalAlign, verticalAlignSet := attributeOf(c, constants.MJMLVerticalAlign)
	backgroundColor := c.resolveAttribute(c, constants.MJMLBackgroundColor)

	// Count mj-column children to calculate width per column
	columnCount := 0
	for _, child := range c.children {
		if _, ok := child.(*MJColumnComponent); ok {
			columnCount++
		}
	}

	widthClass, _ := c.GetColumnClass()

	// As mjml-group's render, the group writes no Outlook cell around itself:
	// a section opens one for it in its row, a wrapper in its own, and the
	// body, a column or a hero none, as in MJML.

	// Root div wrapper (following mjml-group's div style)
	// Note: Class order should match MJML output
	rootDiv := html.NewHTMLTag("div")
	c.addDebugAttribute(rootDiv, "group")
	c.setClassAttribute(rootDiv, widthClass, "mj-outlook-group-fix")

	rootDiv.AddStyle(constants.CSSFontSize, "0"). // Note: "0" not "0px" as MJML writes it
							AddStyle(constants.CSSLineHeight, "0").
							AddStyle(constants.CSSTextAlign, "left").
							AddStyle(constants.CSSDisplay, "inline-block").
							AddStyle(constants.CSSWidth, "100%").
							AddStyle(constants.CSSDirection, direction)

	// vertical-align when a layer sets it: mjml-group has no default.
	if verticalAlignSet {
		rootDiv.AddStyle(constants.CSSVerticalAlign, verticalAlign)
	}

	if backgroundColor != "" {
		rootDiv.AddStyle(constants.CSSBackgroundColor, backgroundColor)
	}

	if err := rootDiv.RenderOpen(w); err != nil {
		return err
	}

	// As mjml-group's render: the Outlook table and row, then each child,
	// a column in an Outlook cell of its own, a raw element as it is, then
	// the end of the row and table; the final merge of adjacent conditional
	// comments joins them (html.OutlookMergeWriter).
	// MJML writes them whatever the children, a single comment included.
	msoTable := html.NewHTMLTag("table")
	if color, set := c.attr(constants.MJMLBackgroundColor); set && color != "none" {
		msoTable.AddAttribute(constants.AttrBgcolor, color)
	}
	msoTable.AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, "0").
		AddAttribute(constants.AttrCellSpacing, "0").
		AddAttribute(constants.AttrRole, "presentation")
	if err := renderMSOTagOpen(w, msoTable); err != nil {
		return err
	}
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
		if columnComp, ok := child.(*MJColumnComponent); ok {

			// A column without width takes an equal share of the group, from
			// the sibling counts of its context (GetParsedWidth), and its
			// mobile width from InsideGroup (mobileWidth), as in MJML.

			// MSO conditional TD for each column: the column's own width in
			// the group, as mjml-group's getElementWidth takes it
			// (getWidthAsPixel, then widthParser, which reads pixels with
			// parseInt), and its vertical alignment
			// (999999999999px stays whole, as JavaScript keeps it).
			width := columnComp.GetWidthAsPixel()
			if parsed, ok := parseMJMLWidth(width); ok {
				width = parsed.cssText()
			}
			td := html.NewHTMLTag("td")
			if align, set := attributeOf(columnComp, constants.MJMLAlign); set {
				td.AddStyle(constants.CSSAlign, align) // in the style, as mjml-group writes it
			}
			td.AddStyle(constants.CSSVerticalAlign, columnComp.resolveAttribute(columnComp, constants.MJMLVerticalAlign)).
				AddStyle(constants.CSSWidth, width)
			if err := renderMSOTagOpen(w, td); err != nil {
				return err
			}

			// Render column content with padding support table wrapper
			if err := child.Render(w); err != nil {
				return err
			}

			if err := html.RenderMSOConditional(w, "</td>"); err != nil {
				return err
			}
			continue
		}
		// Any other child (a misplaced mj-text, mj-section, mj-group…) is
		// rendered as mjml-group renders it, with a warning: in an Outlook
		// cell with the child's align and vertical-align when it has them,
		// and the width of outlookCellWidth.
		td := html.NewHTMLTag("td")
		if align, set := attributeOf(child, constants.MJMLAlign); set {
			td.AddStyle(constants.CSSAlign, align)
		}
		if verticalAlign, set := attributeOf(child, constants.MJMLVerticalAlign); set {
			td.AddStyle(constants.CSSVerticalAlign, verticalAlign)
		}
		if width, ok := c.outlookCellWidth(child); ok {
			td.AddStyle(constants.CSSWidth, width)
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
	if err := html.RenderMSOConditional(w, "</tr></table>"); err != nil {
		return err
	}

	// Close MSO conditional table
	// Close root div
	return rootDiv.RenderClose(w)
}

// attr returns the group's attribute and whether a layer or the default sets
// it.
func (c *MJGroupComponent) attr(name string) (string, bool) {
	return attributeOf(c, name)
}
