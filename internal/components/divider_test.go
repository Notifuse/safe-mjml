package components

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestDividerOutlookWidth checks the width of the divider's Outlook table
// in a 600px column with the default 25px paddings, as mjml-divider's
// getOutlookWidth: parseInt of the percentage that widthParser has
// already read as a number, which JavaScript writes with an exponent from
// 1e21 (1e+21 reads as 1). The values are MJML 5.4.1's.
func TestDividerOutlookWidth(t *testing.T) {
	tests := []struct{ width, want string }{
		{"50%", "275px"},
		{"9" + strings.Repeat("0", 20) + "%", "4.95e+21px"},
		{"1" + strings.Repeat("0", 21) + "%", "5.5px"},
		{strings.Repeat("9", 21) + "%", "5.5px"},
		{"125" + strings.Repeat("0", 19) + "%", "5.5px"},
		{"5" + strings.Repeat("0", 307) + "%", "27.5px"},
		{"-5" + strings.Repeat("0", 307) + "%", "-27.5px"},
		{"100px", "100px"},
		{"100", "100"},
		{"3em", "550px"},
		{" %", "550px"},
	}
	for _, tt := range tests {
		node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-divider"}, Attrs: []xml.Attr{{Name: xml.Name{Local: "width"}, Value: tt.width}}}
		c := NewMJDividerComponent(node, &options.RenderOpts{})
		c.SetContext(ChildContext{ContainerWidth: 600})
		if got := c.outlookWidth(tt.width); got != tt.want {
			t.Errorf("width %.24s: outlookWidth() = %q, want %q", tt.width, got, tt.want)
		}
	}
}
