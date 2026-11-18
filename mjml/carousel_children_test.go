package mjml

import (
	"strings"
	"testing"
)

// TestCarouselChildrenMJMLFailsOn pins how a carousel numbers its images
// around a child MJML 5.4.1 fails on ("component.renderRadio is not a
// function"), so with no reference: an mj-raw or an mj-text is a child
// element, counted as an unknown tag is (the second image is radio-3, with
// three labels each side), and a comment is not a child element there.
func TestCarouselChildrenMJMLFailsOn(t *testing.T) {
	tests := []struct {
		name, between string
		radio         string
		labels        int
	}{
		{"mj-text", "<mj-text>x</mj-text>", "radio-3", 3},
		{"mj-raw", "<mj-raw><p>x</p></mj-raw>", "radio-3", 3},
		{"comment", "<!-- x -->", "radio-2", 2},
	}
	for _, tt := range tests {
		doc := `<mjml><mj-body><mj-section><mj-column><mj-carousel>` +
			`<mj-carousel-image src="https://example.com/1.png" />` + tt.between +
			`<mj-carousel-image src="https://example.com/2.png" />` +
			`</mj-carousel></mj-column></mj-section></mj-body></mjml>`
		out, err := Render(doc)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !strings.Contains(out, "-"+tt.radio+`"`) {
			t.Errorf("%s: no image numbered %s", tt.name, tt.radio)
		}
		if got := strings.Count(out, `class="mj-carousel-next mj-carousel-next-`); got != tt.labels {
			t.Errorf("%s: %d next labels, want %d", tt.name, got, tt.labels)
		}
	}
}
