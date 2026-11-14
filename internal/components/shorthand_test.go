package components

import (
	"encoding/xml"
	"math"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

func TestShorthandSide(t *testing.T) {
	tests := []struct {
		value string
		side  boxSide
		want  int
	}{
		{"", sideLeft, 0},
		{"20px", sideLeft, 20},
		{"10px 20px", sideLeft, 20},
		{"10px 20px", sideTop, 10},
		{"10px 20px 30px", sideRight, 20},
		{"10px 20px 30px", sideBottom, 30},
		{"10px 20px 30px 40px", sideLeft, 40},
		{"10px 20px 30px 40px", sideRight, 20},
		{"10px 20px 30px 40px 50px", sideLeft, 40},
		{"1.5px", sideLeft, 1},
		{"auto", sideLeft, 0},
		{"-5px 0", sideTop, -5},
		{"0 99999999999999999999px", sideLeft, maxLength},
	}
	for _, tt := range tests {
		if got := shorthandSide(tt.value, tt.side); got != tt.want {
			t.Errorf("shorthandSide(%q, %d) = %d, want %d", tt.value, tt.side, got, tt.want)
		}
	}
}

func TestBorderWidth(t *testing.T) {
	tests := []struct {
		border string
		want   int
	}{
		{"", 0},
		{"none", 0},
		{"2px solid #000", 2},
		{"solid 3px red", 3},
		{"1.5px dashed", 1},
		{"#123 4px", 4},
		{"99999999999999999999px solid", maxLength},
	}
	for _, tt := range tests {
		if got := borderWidth(tt.border); got != tt.want {
			t.Errorf("borderWidth(%q) = %d, want %d", tt.border, got, tt.want)
		}
	}
}

func TestLengthFromFloat(t *testing.T) {
	nan := 0.0
	nan = nan / nan
	tests := []struct {
		v    float64
		want int
	}{
		{0, 0}, {199.98, 199}, {-3.5, -3}, {1e20, maxLength}, {-1e20, -maxLength}, {nan, 0},
	}
	for _, tt := range tests {
		if got := lengthFromFloat(tt.v); got != tt.want {
			t.Errorf("lengthFromFloat(%v) = %d, want %d", tt.v, got, tt.want)
		}
	}
}

func TestColumnWidthInZeroWidthContainer(t *testing.T) {
	node := func(width string) *parser.MJMLNode {
		n := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-column"}}
		n.Attrs = []xml.Attr{{Name: xml.Name{Local: "width"}, Value: width}}
		return n
	}
	pixel := NewMJColumnComponent(node("200px"), &options.RenderOpts{})
	pixel.SetContext(ChildContext{ContainerWidth: 0, Siblings: 1})
	if got := pixel.GetEffectiveWidth(); got != 200 {
		t.Errorf("a 200px column in a 0px container is %dpx wide, want 200 as written", got)
	}
	percent := NewMJColumnComponent(node("50%"), &options.RenderOpts{})
	percent.SetContext(ChildContext{ContainerWidth: 0, Siblings: 1})
	if got := percent.GetEffectiveWidth(); got != 0 {
		t.Errorf("a 50%% column in a 0px container is %dpx wide, want 0", got)
	}
}

func TestSumLengths(t *testing.T) {
	tests := []struct {
		lengths []int
		want    int
	}{
		{[]int{600, -25, -25}, 550},
		{[]int{maxLength, maxLength}, maxLength},
		{[]int{0, -maxLength, -maxLength}, -maxLength},
		{[]int{maxLength, -maxLength, -maxLength, maxLength}, 0},
		{nil, 0},
	}
	for _, tt := range tests {
		if got := sumLengths(tt.lengths...); got != tt.want {
			t.Errorf("sumLengths(%v) = %d, want %d", tt.lengths, got, tt.want)
		}
	}
}

// TestJSParseInt checks jsParseInt against what JavaScript prints for
// String(parseInt(s, 10)).
func TestJSParseInt(t *testing.T) {
	for s, want := range map[string]string{
		"300px":        "300",
		"250.7":        "250",
		"  +12em":      "12",
		"-7px":         "-7",
		"-0":           "0",
		"9999999999px": "9999999999",
		"":             "NaN",
		"px":           "NaN",
		"- 5":          "NaN",
		"0012":         "12",
		// Rounded once, as JavaScript does; digit-by-digit accumulation gave
		// ...8500 on arm64 (fused multiply-add) and ...8300 on amd64.
		"1074309632341838545px":          "1074309632341838600",
		"99999999999999999999999px":      "1e+23",
		"1e3px":                          "1",
		"123456789012345678901234567890": "1.2345678901234568e+29",
	} {
		if got := jsParseInt(s); got != want {
			t.Errorf("jsParseInt(%q) = %q, want %q", s, got, want)
		}
	}
}

// TestWidthUnit checks widthUnit against mjml-core's widthParser
// (/[\d.,]*(\D*)$/, "px" when the unit is empty).
func TestWidthUnit(t *testing.T) {
	for width, want := range map[string]string{
		"77%":   "%",
		"300px": "px",
		"200":   "px",
		"":      "px",
		"10em":  "em",
		"1.5em": "em",
		"5.px":  "px",
		"77 %":  " %",
		"auto":  "auto",
		".px":   "px",
	} {
		if got := widthUnit(width); got != want {
			t.Errorf("widthUnit(%q) = %q, want %q", width, got, want)
		}
	}
}

func TestJSPixels(t *testing.T) {
	width, percent := 249.0, 77.0 // computed at run time, as JavaScript does
	for _, tt := range []struct {
		v    float64
		want string
	}{
		{0, "0px"},
		{math.Copysign(0, -1), "0px"},
		{99.33, "99.33px"},
		{width * (percent / 100), "191.73000000000002px"},
	} {
		if got := jsPixels(tt.v); got != tt.want {
			t.Errorf("jsPixels(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

// TestJSNumber checks jsNumber against what JavaScript prints for
// String(v), checked with node.
func TestJSNumber(t *testing.T) {
	for _, tt := range []struct {
		v    float64
		want string
	}{
		{1e21, "1e+21"},
		{1.5e21, "1.5e+21"},
		{1e-7, "1e-7"},
		{1.25e-7, "1.25e-7"},
		{0.000001, "0.000001"},
		{123.456, "123.456"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
		{1e301, "1e+301"},
		{math.Copysign(0, -1), "0"},
	} {
		if got := jsNumber(tt.v); got != tt.want {
			t.Errorf("jsNumber(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

// TestSideAttributeNames checks the precomputed longhand names against the
// shorthand name and the side's suffix.
func TestSideAttributeNames(t *testing.T) {
	for attribute, names := range sideAttributeNames {
		for side := sideTop; side <= sideLeft; side++ {
			if want := attribute + side.suffix(); names[side] != want {
				t.Errorf("sideAttribute(%q, %d) = %q, want %q", attribute, side, names[side], want)
			}
		}
	}
	if got := sideAttribute("margin", sideLeft); got != "margin-left" {
		t.Errorf(`sideAttribute("margin", sideLeft) = %q`, got)
	}
}

// parseIntText writes a value back as JavaScript's parseInt reads it, and
// keeps a value parseInt reads as NaN as written.
func TestParseIntText(t *testing.T) {
	for in, want := range map[string]string{
		"120.7px": "120", "99.99px": "99", "045px": "45", "0px": "0", "-0px": "0", " 12px": "12",
		"-5px": "-5", "+7": "7", "abc": "abc", "": "", "px12": "px12",
		"123456789012345678901234567890px": "123456789012345678901234567890",
	} {
		if got := parseIntText(in); got != want {
			t.Errorf("parseIntText(%q) = %q, want %q", in, got, want)
		}
	}
}
