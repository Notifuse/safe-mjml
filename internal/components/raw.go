package components

import (
	"io"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// MJRawComponent represents an mj-raw component
// It outputs its inner content exactly as provided without any additional wrappers.
type MJRawComponent struct {
	*BaseComponent
	// Content stores the original inner HTML captured during parsing
	Content string
}

// NewMJRawComponent creates a new mj-raw component
func NewMJRawComponent(node *parser.MJMLNode, opts *options.RenderOpts) *MJRawComponent {
	return &MJRawComponent{
		BaseComponent: NewBaseComponent(node, opts),
		Content:       node.Text,
	}
}

// GetTagName returns the component's tag name
func (c *MJRawComponent) GetTagName() string { return "mj-raw" }

// IsRawElement indicates this component should be treated as a raw element
func (c *MJRawComponent) IsRawElement() bool { return true }

// isEmptyRaw reports whether comp is an mj-raw that renders nothing: Render
// trims the content and only removes whitespace between tags, so the output
// is empty exactly when the trimmed content is.
func isEmptyRaw(comp Component) bool {
	raw, ok := comp.(*MJRawComponent)
	return ok && strings.TrimSpace(raw.Content) == ""
}

// GetDefaultAttribute returns default values for the component's attributes
func (c *MJRawComponent) GetDefaultAttribute(name string) string { return "" }

// Render writes the content as written, trimmed, as mjml-raw renders it
// (getContent): its whitespace, self-closing tags and conditional comments
// are kept as they are.
func (c *MJRawComponent) Render(w io.StringWriter) error {
	_, err := w.WriteString(strings.TrimSpace(c.Content))
	return err
}
