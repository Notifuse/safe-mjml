package mjml

import (
	"regexp"
	"strings"
	"testing"
)

// TestSocialElementWithoutIconSize pins an mj-social-element that gets no
// icon-size: MJML 5.4.1 has no default for it on the element (mj-social
// passes its 20px down), so outside an mj-social, or with icon-size="", it
// writes no width and no height on the icon's table and cell, and
// width="NaN" on the image, where safe-mjml writes no width (no NaN). Inside
// an mj-social the icon keeps the social's 20px.
func TestSocialElementWithoutIconSize(t *testing.T) {
	icon := regexp.MustCompile(`<img[^>]*twitter-x\.png[^>]*>`)
	tests := []struct {
		name, body string
		width      string // the image's width attribute, "" for none
	}{
		{"outside mj-social", `<mj-social-element name="x">t</mj-social-element>`, ""},
		{"empty icon-size", `<mj-social><mj-social-element name="x" icon-size="">t</mj-social-element></mj-social>`, ""},
		{"inside mj-social", `<mj-social><mj-social-element name="x">t</mj-social-element></mj-social>`, "20"},
	}
	for _, tt := range tests {
		out, err := Render(`<mjml><mj-body><mj-section><mj-column>` + tt.body + `</mj-column></mj-section></mj-body></mjml>`)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		img := icon.FindString(out)
		if img == "" {
			t.Fatalf("%s: no icon in %s", tt.name, out)
		}
		if strings.Contains(out, "NaN") {
			t.Errorf("%s: NaN written", tt.name)
		}
		switch {
		case tt.width == "" && strings.Contains(img, "width="):
			t.Errorf("%s: icon %s has a width", tt.name, img)
		case tt.width != "" && !strings.Contains(img, `width="`+tt.width+`"`):
			t.Errorf("%s: icon %s, want width %s", tt.name, img, tt.width)
		}
		if tt.width == "" && (strings.Contains(out, "width:20px") || strings.Contains(out, "height:20px")) {
			t.Errorf("%s: 20px icon size written", tt.name)
		}
	}
}
