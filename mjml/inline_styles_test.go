package mjml

import (
	"os"
	"strings"
	"testing"
)

// TestInlineStyleAttributeWithoutDeclarations pins the two documents of
// testdata/liquid that MJML 5.4.1 cannot render (rejected.json): an inline
// mj-style rule reaches an element whose style attribute has no declaration
// juice reads ("{{ styles.paragraph }}", or a {% if %} block that juice does
// not protect), and juice 11 throws on it. safe-mjml renders them as it did
// with juice 10 and keeps the template blocks (Pierre's decision,
// 2026-11-17; docs/differences-from-mjml.md).
func TestInlineStyleAttributeWithoutDeclarations(t *testing.T) {
	tests := []struct{ name, want string }{
		{"text-style-if--inline", `<p class="liq" style="font-weight: bold; {% if contact.vip %}color: #b8860b; {% endif %}margin: 0;">Hello</p>`},
		{"text-style-only-variable--inline", `<p class="liq" style="font-weight: bold; {{ styles.paragraph }};">Hello</p>`},
	}
	for _, tt := range tests {
		source, err := os.ReadFile("testdata/liquid/" + tt.name + ".mjml")
		if err != nil {
			t.Fatal(err)
		}
		got, err := Render(string(source))
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: the paragraph is not\n%s\nin\n%s", tt.name, tt.want, got)
		}
	}
}
