package components

import (
	"io"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJSpacerComponent represents the mj-spacer component
type MJSpacerComponent struct {
	*BaseComponent
}

func NewMJSpacerComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJSpacerComponent {
	return &MJSpacerComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJSpacerComponent) Render(w io.StringWriter) error {
	height := c.resolveAttribute(c, constants.MJMLHeight)

	// Create div with height and line-height, containing thin space character
	div := html.NewHTMLTag("div").
		AddStyle(constants.CSSHeight, height).
		AddStyle(constants.CSSLineHeight, height)

	if err := div.RenderOpen(w); err != nil {
		return err
	}

	// Add thin space character (&#8202;)
	// The thin space character is used here to ensure the spacer div renders with the correct height in email clients.
	// Some clients ignore empty divs or collapse whitespace, and using &nbsp; or a regular space may result in unwanted extra height or layout issues.
	if _, err := w.WriteString("&#8202;"); err != nil {
		return err
	}

	return div.RenderClose(w)
}

func (c *MJSpacerComponent) GetTagName() string {
	return "mj-spacer"
}

func (c *MJSpacerComponent) GetDefaultAttribute(name string) string {
	switch name {
	case constants.MJMLHeight:
		return "20px"
	default:
		return ""
	}
}
