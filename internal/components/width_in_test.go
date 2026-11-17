package components

import (
	"testing"

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
