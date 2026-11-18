package components

import (
	"math"
	"testing"
)

// A width is read as mjml-core's widthParser reads it for mj-column and
// mj-group: parseFloat for a "%" width, parseInt for any other unit, which
// is pixels, and the unit as written for the media queries. Usual widths
// read as they always did.
func TestParseMJMLWidth(t *testing.T) {
	for _, tc := range []struct {
		in, size, css string
	}{
		{"300px", "300px", "300px"},
		{"200.9px", "200px", "200px"},
		{"300", "300px", "300px"},
		{"33.5%", "33.5%", "33.5%"},
		{"50%", "50%", "50%"},
		{"300PX", "300px", "300PX"},
		{"150Px", "150px", "150Px"},
		{"50 %", "50px", "50 %"},
		{"66.5 %", "66px", "66 %"},
		{"40%;", "40px", "40%;"},
		{" 12px", "12px", "12px"},
		{"1e2%", "100%", "100%"},
	} {
		w, ok := parseMJMLWidth(tc.in)
		if !ok || w.size.String() != tc.size || w.cssText() != tc.css {
			t.Errorf("parseMJMLWidth(%q) = %v %q %v, want %q %q", tc.in, w.size.String(), w.cssText(), ok, tc.size, tc.css)
		}
	}
	for _, in := range []string{"abc", "%", "px", "auto"} {
		if w, ok := parseMJMLWidth(in); ok {
			t.Errorf("parseMJMLWidth(%q) = %v, want no width (NaN in MJML)", in, w)
		}
	}
}

// jsMathRound and jsToFixed6 are mjml-column's normalizePxValue and
// normalizeUnitValue (MJML 5): Math.round, and Number(x.toFixed(6)). The
// expected values are Node's (String(Math.round(x)) and
// String(Number(x.toFixed(6)))), where Go's math.Round, math.Floor(x+0.5)
// and strconv.FormatFloat(x, 'f', 6, 64) differ.
func TestJSMathRound(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{0.5, "1"},
		{1.5, "2"},
		{2.5, "3"},
		{0.49999999999999994, "0"}, // math.Floor(x+0.5) gives 1
		{1<<52 + 1, "4503599627370497"},
		{-0.5, "0"}, // -0; math.Round gives -1
		{-1.5, "-1"},
		{-0.4, "0"},
		{300.5, "301"},
		{199.98, "200"},
		{math.NaN(), "NaN"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
	} {
		if got := jsNumber(jsMathRound(tc.in)); got != tc.want {
			t.Errorf("Math.round(%v) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestJSToFixed6(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{1.0 / 128, "0.007813"}, // an exact tie: FormatFloat rounds to even, 0.007812
		{-1.0 / 128, "-0.007813"},
		{-1e-7, "0"},
		{1e-7, "0"},
		{5e-7, "0"}, // the double is just under 5e-7
		{2.5e-7, "0"},
		{33.3333335, "33.333334"},
		{0.1 + 0.2, "0.3"},
		{100.0 / 3, "33.333333"},
		{49.91680532445923, "49.916805"},
		{50, "50"},
	} {
		if got := jsNumber(jsToFixed6(tc.in)); got != tc.want {
			t.Errorf("Number((%v).toFixed(6)) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
