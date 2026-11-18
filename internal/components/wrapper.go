package components

import (
	"io"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJWrapperComponent represents mj-wrapper
type MJWrapperComponent struct {
	*BaseComponent
}

// NewMJWrapperComponent creates a new mj-wrapper component
func NewMJWrapperComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJWrapperComponent {
	return &MJWrapperComponent{
		BaseComponent: NewBaseComponent(node, opts),
	}
}

func (c *MJWrapperComponent) GetDefaultAttribute(name string) string {
	switch name {
	case "background-position":
		return "top center"
	case "background-repeat":
		return "repeat"
	case "background-size":
		return "auto"
	case "direction":
		return "ltr"
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

// childContainerWidth returns the width the wrapper gives its sections and
// heroes: its width minus its left and right padding and borders, as
// mjml-wrapper, which extends mjml-section, computes it (getBoxWidths),
// never negative. childContext gives it to the children.
func (c *MJWrapperComponent) childContainerWidth() int {
	return nonNegativeWidth(c.BaseComponent, c.boxWidth(c))
}

// Render writes the wrapper as mjml-wrapper does: an mj-section
// (sectionBlock) whose children each sit in an Outlook row of their own.
func (c *MJWrapperComponent) Render(w io.StringWriter) error {
	return sectionBlock{BaseComponent: c.BaseComponent, comp: c, wrapped: c.renderWrappedChildren}.render(w)
}

// renderWrappedChildren writes the children as mjml-wrapper's
// renderWrappedChildren: a raw child as it is, any other in an Outlook row
// and cell as wide as the wrapper's container.
func (c *MJWrapperComponent) renderWrappedChildren(w io.StringWriter) error {
	width := c.containerWidthCSS()
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
		td.AddAttribute(constants.AttrClass, suffixCSSClasses(css, constants.ClassSuffixOutlook)).
			AddAttribute(constants.AttrWidth, width)
		var open strings.Builder
		if err := td.RenderOpen(&open); err != nil {
			return err
		}
		if err := html.RenderMSOConditional(w, "<tr>"+strings.TrimSuffix(open.String(), ">")+" >"); err != nil {
			return err
		}
		if err := child.Render(w); err != nil {
			return err
		}
		if err := html.RenderMSOConditional(w, "</td></tr>"); err != nil {
			return err
		}
	}
	return nil
}

func (c *MJWrapperComponent) GetTagName() string {
	return "mj-wrapper"
}
