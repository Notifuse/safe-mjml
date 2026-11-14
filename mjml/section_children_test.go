package mjml

import (
	"strings"
	"testing"
)

// TestSectionKeepsChildrenNextToSplitColumn checks that a lone column needing
// the split Outlook table does not swallow the other children of its section.
func TestSectionKeepsChildrenNextToSplitColumn(t *testing.T) {
	tests := []struct {
		name     string
		children string
	}{
		{name: "content after the column", children: `<mj-column><mj-text align="right">CR</mj-text></mj-column><mj-text>KEEP-ME</mj-text>`},
		{name: "content before the column", children: `<mj-image src="https://example.com/keep-me.png" alt="KEEP-ME" /><mj-column><mj-text align="right">CR</mj-text></mj-column>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//GIVEN: a section with a right-aligned column and another child.
			template := `<mjml><mj-body><mj-section>` + tt.children + `</mj-section></mj-body></mjml>`

			//WHEN: it is rendered.
			html, err := Render(template)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			//THEN: both the column and the other child are in the output.
			if !strings.Contains(html, "CR") {
				t.Errorf("expected the column content in the output")
			}
			if !strings.Contains(html, "KEEP-ME") {
				t.Errorf("expected the other section child in the output")
			}
		})
	}
}
