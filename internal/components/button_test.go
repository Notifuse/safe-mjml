package components

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestButtonLinkWidth checks the width of the button's link, as
// mjml-button's calculateAWidth: the width less the inner padding and the
// borders, Infinity in MJML for a width parseInt cannot hold, saturated at
// ±2147483647 here (safe-mjml writes no Infinity), and the same on 32-bit
// builds, where the inner paddings, each read as up to 2147483647, used to
// overflow an int.
func TestButtonLinkWidth(t *testing.T) {
	digits400 := "9" + strings.Repeat("0", 399)
	tests := []struct{ width, innerPadding, want string }{
		{"200px", "", "150px"},
		{"17" + strings.Repeat("0", 307) + "px", "", "1.7e+308px"},
		{digits400 + "px", "", "2147483597px"},
		{"-" + digits400 + "px", "", "-2147483697px"},
		{digits400 + "px", "10px " + digits400 + "px", "-2147483647px"},
		{"100px", "10px 9999999999px", "-4294967194px"},
		{"100px", "10px 2147483647px", "-4294967194px"},
		{"50%", "", ""},
	}
	for _, tt := range tests {
		attrs := []xml.Attr{{Name: xml.Name{Local: "width"}, Value: tt.width}}
		if tt.innerPadding != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "inner-padding"}, Value: tt.innerPadding})
		}
		c := NewMJButtonComponent(&parser.MJMLNode{XMLName: xml.Name{Local: "mj-button"}, Attrs: attrs}, &options.RenderOpts{})
		if got := c.linkWidth(tt.width); got != tt.want {
			t.Errorf("width %.20s…, inner-padding %.20s…: linkWidth() = %q, want %q", tt.width, tt.innerPadding, got, tt.want)
		}
	}
}
