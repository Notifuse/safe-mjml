package components

import "testing"

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
