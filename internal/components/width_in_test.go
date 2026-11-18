package components

import (
	"encoding/xml"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
	"github.com/Notifuse/safe-mjml/internal/styles"
)

// TestWidthInSaturatesOverflow checks that a percentage whose pixels
// overflow a float64, Infinity in MJML, is saturated, since safe-mjml writes
// no Infinity width, and that any other width is as MJML computes it.
func TestWidthInSaturatesOverflow(t *testing.T) {
	tests := []struct {
		size      styles.Size
		container float64
		want      float64
	}{
		{styles.NewPercentSize(50), 600, 300},
		{styles.NewPercentSize(1e10), 600, 6e10}, // finite: as MJML writes it
		{styles.NewPercentSize(1.7e308), 600, maxLength},
		{styles.NewPercentSize(-1.7e308), 600, -maxLength},
		{styles.NewPixelSize(250), 600, 250},
	}
	for _, tt := range tests {
		if got := widthIn(tt.size, tt.container); got != tt.want {
			t.Errorf("widthIn(%v, %v) = %v, want %v", tt.size, tt.container, got, tt.want)
		}
	}
}

// TestParseIntWidth checks the container width as MJML's parseInt readers
// read their containerWidth string: truncated, a number written with an
// exponent read as its first digits (1e+21px is 1, 6.01e-7px is 6), the
// body's width as written, and not saturated, where GetEffectiveWidth is.
func TestParseIntWidth(t *testing.T) {
	tests := []struct {
		width     float64
		text      string
		want      float64
		effective int
	}{
		{600, "", 600, 600},
		{601.9, "", 601, 601},
		{6.01e-7, "", 6, 6},
		{1e21, "", 1, 1},
		{2.5e21, "", 2, 2},
		{999999999999, "", 999999999999, maxLength},
		{601.5, "601.5px", 601, 601},
		{1e21, "1000000000000000000000px", 1e21, maxLength},
	}
	for _, tt := range tests {
		bc := NewBaseComponent(&parser.MJMLNode{XMLName: xml.Name{Local: "mj-text"}}, &options.RenderOpts{})
		bc.SetContext(ChildContext{ContainerWidth: tt.width, containerWidthText: tt.text})
		if got := bc.parseIntWidth(); got != tt.want {
			t.Errorf("width %v (%q): parseIntWidth() = %v, want %v", tt.width, tt.text, got, tt.want)
		}
		if got := bc.GetEffectiveWidth(); got != tt.effective {
			t.Errorf("width %v (%q): GetEffectiveWidth() = %v, want %v", tt.width, tt.text, got, tt.effective)
		}
	}
}
