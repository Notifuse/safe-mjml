package components

import (
	"io"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/fonts"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJTableComponent represents the mj-table component
type MJTableComponent struct {
	*BaseComponent
}

func NewMJTableComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJTableComponent {
	return &MJTableComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJTableComponent) Render(w io.StringWriter) error {
	// Get attributes

	// Create table element with styles
	tableTag := html.NewHTMLTag("table").
		// HTML border attribute should always be "0", even with CSS border
		AddAttribute(constants.AttrBorder, "0").
		AddAttribute(constants.AttrCellPadding, c.resolveAttribute(c, constants.MJMLCellPadding)).
		AddAttribute(constants.AttrCellSpacing, c.resolveAttribute(c, constants.MJMLCellSpacing))
	c.addSetAttribute(c, tableTag, constants.AttrRole)
	tableTag.AddAttribute(constants.AttrWidth, c.widthAttribute()).
		AddStyle(constants.CSSColor, c.resolveAttribute(c, constants.MJMLColor)).
		AddStyle(constants.CSSFontFamily, c.resolveAttribute(c, constants.MJMLFontFamily)).
		AddStyle(constants.CSSFontSize, c.resolveAttribute(c, constants.MJMLFontSize)).
		AddStyle(constants.CSSLineHeight, c.resolveAttribute(c, constants.MJMLLineHeight)).
		AddStyle(constants.CSSTableLayout, c.resolveAttribute(c, constants.MJMLTableLayout)).
		AddStyle(constants.CSSWidth, c.resolveAttribute(c, constants.MJMLWidth)).
		AddStyle(constants.CSSBorder, c.resolveAttribute(c, constants.MJMLBorder)) // Use the actual border value for CSS

	// Cell spacing only shows when borders are separate, so MJML 5.4.1 sets
	// border-collapse:separate whenever cellspacing is positive.
	if hasPositiveCellSpacing(c.resolveAttribute(c, constants.MJMLCellSpacing)) {
		tableTag.AddStyle(constants.CSSBorderCollapse, constants.BorderCollapseSeparate)
	}

	if err := tableTag.RenderOpen(w); err != nil {
		return err
	}
	if err := c.writeBlank(w); err != nil {
		return err
	}

	// Write the inner HTML content (TR, TH, TD elements)
	if err := c.writeInnerTableContent(w); err != nil {
		return err
	}

	if err := tableTag.RenderClose(w); err != nil {
		return err
	}

	return nil
}

func (c *MJTableComponent) GetTagName() string {
	return "mj-table"
}

func (c *MJTableComponent) GetDefaultAttribute(name string) string {
	switch name {
	case "align":
		return "left"
	case constants.MJMLBorder:
		return "none"
	case "cellpadding":
		return "0"
	case "cellspacing":
		return "0"
	case "color":
		return "#000000"
	case "font-family":
		return fonts.DefaultFontStack
	case "font-size":
		return "13px"
	case "line-height":
		return "22px"
	case "padding":
		return "10px 25px"
	case "table-layout":
		return "auto"
	case "width":
		return "100%"
	default:
		return ""
	}
}

// writeInnerTableContent writes the rows as written: the parser keeps the
// content raw, as MJML does for an ending tag.
func (c *MJTableComponent) writeInnerTableContent(w io.StringWriter) error {
	if c.Node.Text != "" {
		_, err := w.WriteString(strings.TrimSpace(c.Node.Text))
		return err
	}

	return nil
}

// hasPositiveCellSpacing reports whether a cellspacing value is a number
// greater than zero, following mjml-table's hasCellspacing: every character
// other than digits and dots is ignored ("4px" counts as 4).
func hasPositiveCellSpacing(cellspacing string) bool {
	digits := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, cellspacing)
	// Like JavaScript's parseFloat, read only the leading number ("1.2.3" is 1.2).
	if first := strings.IndexByte(digits, '.'); first >= 0 {
		if second := strings.IndexByte(digits[first+1:], '.'); second >= 0 {
			digits = digits[:first+1+second]
		}
	}
	value, err := strconv.ParseFloat(digits, 64)
	return err == nil && value > 0
}

// widthAttribute returns the table's width attribute, as mjml-table's
// getWidth: "auto" and a percentage as written, any other width read with
// parseInt (300px gives 300), with parseIntAttribute.
func (c *MJTableComponent) widthAttribute() string {
	width := c.resolveAttribute(c, constants.MJMLWidth)
	if width == "auto" || widthUnit(width) == "%" {
		return width
	}
	return parseIntAttribute(width)
}
