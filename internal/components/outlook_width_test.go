package components

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestOutlookWidthReadsTheWidthAgain checks the width of the Outlook cell
// of a column and of a group in a 600px section, as getWidthAsPixel writes
// it: it reads getParsedWidth(true), the width as JavaScript writes the
// number widthParser read, with widthParser again, so a pixel width of
// 1e21 or more (1e+21px) is its first digit. The values are MJML 5.4.1's.
func TestOutlookWidthReadsTheWidthAgain(t *testing.T) {
	tests := []struct{ width, want string }{
		{"200.9px", "200px"},
		{"9" + strings.Repeat("0", 20) + "px", "900000000000000000000px"},
		{"1" + strings.Repeat("0", 21) + "px", "1px"},
		{strings.Repeat("9", 21) + "px", "1px"},
		{"25" + strings.Repeat("0", 20) + "px", "2px"},
		{"17" + strings.Repeat("0", 307) + "px", "1px"},
		{"-17" + strings.Repeat("0", 307) + "px", "-1px"},
		{"1" + strings.Repeat("0", 21) + "PX", "1px"},
		{"1" + strings.Repeat("0", 21) + "em", "1px"},
		{"1" + strings.Repeat("0", 21) + "%", "6e+21px"},
		{"50%", "300px"},
	}
	for _, tag := range []string{"mj-column", "mj-group"} {
		for _, tt := range tests {
			node := &parser.MJMLNode{XMLName: xml.Name{Local: tag}, Attrs: []xml.Attr{{Name: xml.Name{Local: "width"}, Value: tt.width}}}
			var got string
			if tag == "mj-column" {
				c := NewMJColumnComponent(node, &options.RenderOpts{})
				c.SetContext(ChildContext{ContainerWidth: 600, Siblings: 1, InSection: true})
				got = c.GetWidthAsPixel()
			} else {
				c := NewMJGroupComponent(node, &options.RenderOpts{})
				c.SetContext(ChildContext{ContainerWidth: 600, Siblings: 1, InSection: true})
				got = c.GetWidthAsPixel()
			}
			if got != tt.want {
				t.Errorf("%s width %.24s: GetWidthAsPixel() = %q, want %q", tag, tt.width, got, tt.want)
			}
		}
	}
}
