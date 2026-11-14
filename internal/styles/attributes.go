package styles

import (
	"fmt"
	"strconv"
)

// NormalizeColor expands shorthand hex color values (#rgb) to their full
// six-character form (#rrggbb). Values that are already in full hex form or
// use other CSS color syntaxes (e.g., rgba(), named colors) are returned
// unchanged.
func NormalizeColor(value string) string {
	if len(value) == 4 && value[0] == '#' && isHexColor(value[1:]) {
		r := value[1]
		g := value[2]
		b := value[3]
		return fmt.Sprintf("#%c%c%c%c%c%c", r, r, g, g, b, b)
	}
	return value
}

// isHexColor checks if a string contains only valid hex color characters
func isHexColor(s string) bool {
	if len(s) != 3 && len(s) != 6 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// Size represents CSS size values (width, height) that can be in pixels or percentages.
type Size struct {
	value   float64
	isPixel bool // true for pixels, false for percentages
}

// NewPixelSize creates a new pixel-based Size
func NewPixelSize(value float64) Size {
	return Size{value: value, isPixel: true}
}

// NewPercentSize creates a new percentage-based Size
func NewPercentSize(value float64) Size {
	return Size{value: value, isPixel: false}
}

// Value returns the numeric value of the size
func (s Size) Value() float64 {
	return s.value
}

// IsPercent returns true if this is a percentage-based size
func (s Size) IsPercent() bool {
	return !s.isPixel
}

// String returns the CSS representation of the size
func (s Size) String() string {
	if s.isPixel {
		return fmt.Sprintf("%.0fpx", s.value)
	}
	formatted := strconv.FormatFloat(s.value, 'g', -1, 64)
	if formatted == "" {
		formatted = "0"
	}
	return formatted + "%"
}
