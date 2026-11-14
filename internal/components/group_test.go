package components

import (
	"testing"

	"github.com/Notifuse/safe-mjml/internal/globals"
	"github.com/Notifuse/safe-mjml/internal/options"
)

// newTestGroup builds an mj-group placed in a section of containerWidth pixels
// with the given sibling counts, as the section sets them before rendering.
func newTestGroup(attrs map[string]string, ga *globals.GlobalAttributes, containerWidth, siblings, rawSiblings int) *MJGroupComponent {
	opts := &options.RenderOpts{GlobalAttributes: ga}
	group := NewMJGroupComponent(nodeWithAttrs("mj-group", attrs), opts)
	group.SetContext(ChildContext{ContainerWidth: float64(containerWidth), Siblings: siblings, RawSiblings: rawSiblings})
	return group
}

// TestMJGroupWidth checks the width a group takes in its section, following
// mjml-group: an explicit width (element, mj-class or mj-attributes) wins,
// otherwise the group gets an equal share of the non-raw siblings.
func TestMJGroupWidth(t *testing.T) {
	tests := []struct {
		name          string
		attrs         map[string]string
		globals       *globals.GlobalAttributes
		siblings      int
		rawSiblings   int
		wantParsed    string
		wantClass     string
		wantRuleWidth string
		wantPixel     string
	}{
		{
			name:          "alone in its section takes the full width",
			siblings:      1,
			wantParsed:    "100%",
			wantClass:     "mj-column-per-100",
			wantRuleWidth: "100%",
			wantPixel:     "600px",
		},
		{
			name:          "next to one sibling takes half",
			siblings:      2,
			wantParsed:    "50%",
			wantClass:     "mj-column-per-50",
			wantRuleWidth: "50%",
			wantPixel:     "300px",
		},
		{
			name:          "next to two siblings takes a third, class uses the integer part",
			siblings:      3,
			wantParsed:    "33.333333333333336%",
			wantClass:     "mj-column-per-33",
			wantRuleWidth: "33.333333333333336%",
			wantPixel:     "200px",
		},
		{
			name:          "raw siblings do not take a share",
			siblings:      3,
			rawSiblings:   2,
			wantParsed:    "100%",
			wantClass:     "mj-column-per-100",
			wantRuleWidth: "100%",
			wantPixel:     "600px",
		},
		{
			name:          "explicit percentage",
			attrs:         map[string]string{"width": "60%"},
			siblings:      2,
			wantParsed:    "60%",
			wantClass:     "mj-column-per-60",
			wantRuleWidth: "60%",
			wantPixel:     "360px",
		},
		{
			name:          "decimal percentage keeps the exact rule but an integer class",
			attrs:         map[string]string{"width": "33.5%"},
			siblings:      2,
			wantParsed:    "33.5%",
			wantClass:     "mj-column-per-33",
			wantRuleWidth: "33.5%",
			wantPixel:     "201px",
		},
		{
			// MJML 4.18 writes the section's Outlook cell for this group as
			// width:187.5px: getWidthAsPixel keeps the fraction.
			name:          "pixel width on a fractional percentage share keeps its fraction",
			attrs:         map[string]string{"width": "31.25%"},
			siblings:      1,
			wantParsed:    "31.25%",
			wantClass:     "mj-column-per-31",
			wantRuleWidth: "31.25%",
			wantPixel:     "187.5px",
		},
		{
			name:          "explicit pixels",
			attrs:         map[string]string{"width": "400px"},
			siblings:      2,
			wantParsed:    "400px",
			wantClass:     "mj-column-px-400",
			wantRuleWidth: "400px",
			wantPixel:     "400px",
		},
		{
			name:          "fractional pixels are truncated like MJML's parseInt",
			attrs:         map[string]string{"width": "400.7px"},
			siblings:      2,
			wantParsed:    "400px",
			wantClass:     "mj-column-px-400",
			wantRuleWidth: "400px",
			wantPixel:     "400px",
		},
		{
			name:          "width from mj-class",
			attrs:         map[string]string{"mj-class": "half"},
			globals:       globalsWithClass("half", "width", "40%"),
			siblings:      2,
			wantParsed:    "40%",
			wantClass:     "mj-column-per-40",
			wantRuleWidth: "40%",
			wantPixel:     "240px",
		},
		{
			name:          "width from mj-attributes",
			globals:       globalsWithComponentDefault("mj-group", "width", "300px"),
			siblings:      2,
			wantParsed:    "300px",
			wantClass:     "mj-column-px-300",
			wantRuleWidth: "300px",
			wantPixel:     "300px",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//GIVEN: a group in a 600px section with the given siblings and attributes.
			group := newTestGroup(tt.attrs, tt.globals, 600, tt.siblings, tt.rawSiblings)

			//WHEN: its width, class and Outlook pixel width are computed.
			parsed := group.GetParsedWidth()
			className, ruleSize := group.GetColumnClass()
			pixel := group.GetWidthAsPixel()

			//THEN: they match what MJML computes for the same group.
			if parsed.String() != tt.wantParsed {
				t.Errorf("GetParsedWidth() = %q, want %q", parsed.String(), tt.wantParsed)
			}
			if className != tt.wantClass {
				t.Errorf("GetColumnClass() class = %q, want %q", className, tt.wantClass)
			}
			if ruleSize != tt.wantRuleWidth {
				t.Errorf("GetColumnClass() rule width = %q, want %q", ruleSize, tt.wantRuleWidth)
			}
			if pixel != tt.wantPixel {
				t.Errorf("GetWidthAsPixel() = %q, want %q", pixel, tt.wantPixel)
			}
		})
	}
}

// TestMJGroupHasNoDefaultWidth pins MJML's mj-group defaults, which do not
// include a width: a width-less group shares the section with its siblings.
func TestMJGroupHasNoDefaultWidth(t *testing.T) {
	//GIVEN: a group without attributes.
	group := newTestGroup(nil, nil, 600, 1, 0)

	//WHEN: its default width is read.
	got := group.GetDefaultAttribute("width")

	//THEN: there is none.
	if got != "" {
		t.Errorf("GetDefaultAttribute(width) = %q, want empty", got)
	}
}
