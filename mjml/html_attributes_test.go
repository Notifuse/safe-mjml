package mjml

import (
	"strings"
	"testing"
)

// TestHTMLAttributesInvalidSelector checks that an mj-selector whose path
// cheerio cannot read sets nothing and gives a warning on its line, where
// MJML fails, and that the other selectors still apply.
func TestHTMLAttributesInvalidSelector(t *testing.T) {
	result, err := Compile(`<mjml>
  <mj-head>
    <mj-html-attributes>
      <mj-selector path="td["><mj-html-attribute name="data-bad">1</mj-html-attribute></mj-selector>
      <mj-selector path=".x td"><mj-html-attribute name="data-good">2</mj-html-attribute></mj-selector>
    </mj-html-attributes>
  </mj-head>
  <mj-body><mj-section css-class="x"><mj-column><mj-text>t</mj-text></mj-column></mj-section></mj-body>
</mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "data-bad") || !strings.Contains(result.HTML, `data-good="2"`) {
		t.Errorf("the invalid selector set an attribute, or the valid one none:\n%s", result.HTML)
	}
	var found bool
	for _, w := range result.Warnings {
		if w.Line == 4 && strings.Contains(w.Message, `mj-selector path "td[" is not a valid selector`) {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning on line 4 for the invalid selector: %v", result.Warnings)
	}
}
