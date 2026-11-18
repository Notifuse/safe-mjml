package components

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestHtmlAttributesWriteNothing checks that mj-html-attributes writes
// nothing where it stands: its attributes are set on the body after it is
// written, as mjml-head-html-attributes registers them in its handler.
func TestHtmlAttributesWriteNothing(t *testing.T) {
	node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-html-attributes"}}
	c := NewMJHtmlAttributesComponent(node, &options.RenderOpts{})
	if got := c.GetTagName(); got != "mj-html-attributes" {
		t.Errorf("GetTagName() = %q", got)
	}
	var out strings.Builder
	if err := c.Render(&out); err != nil || out.Len() != 0 {
		t.Errorf("Render wrote %q, %v; want nothing", out.String(), err)
	}
}
