package parser

import (
	"strings"
	"testing"
)

func TestParseMJML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantTag string
		wantErr bool
	}{
		{
			name:    "basic mjml",
			input:   `<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>`,
			wantTag: "mjml",
			wantErr: false,
		},
		{
			name:    "with attributes",
			input:   `<mjml version="4.0"><mj-head><mj-title>Test</mj-title></mj-head></mjml>`,
			wantTag: "mjml",
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			wantTag: "",
			wantErr: true,
		},
		{
			name:    "invalid xml",
			input:   `<mjml><mj-body><mj-section></mj-body></mjml>`,
			wantTag: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseMJML(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseMJML() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseMJML() error = %v", err)
				return
			}

			if node.GetTagName() != tt.wantTag {
				t.Errorf("ParseMJML() tag = %v, want %v", node.GetTagName(), tt.wantTag)
			}
		})
	}
}

func TestMJMLNode_GetAttribute(t *testing.T) {
	input := `<mjml version="4.0" lang="en"><mj-head></mj-head></mjml>`
	node, err := ParseMJML(input)
	if err != nil {
		t.Fatalf("ParseMJML() error = %v", err)
	}

	tests := []struct {
		name string
		attr string
		want string
	}{
		{"existing attribute", "version", "4.0"},
		{"another existing attribute", "lang", "en"},
		{"non-existing attribute", "nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := node.GetAttribute(tt.attr)
			if got != tt.want {
				t.Errorf("GetAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMJMLNode_FindFirstChild(t *testing.T) {
	input := `<mjml><mj-head><mj-title>Test</mj-title></mj-head><mj-body><mj-text>Hello</mj-text></mj-body></mjml>`
	node, err := ParseMJML(input)
	if err != nil {
		t.Fatalf("ParseMJML() error = %v", err)
	}

	tests := []struct {
		name    string
		tagName string
		wantTag string
		wantNil bool
	}{
		{"find head", "mj-head", "mj-head", false},
		{"find body", "mj-body", "mj-body", false},
		{"find non-existing", "mj-section", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := node.FindFirstChild(tt.tagName)

			if tt.wantNil {
				if child != nil {
					t.Errorf("FindFirstChild() expected nil but got %v", child.GetTagName())
				}
				return
			}

			if child == nil {
				t.Errorf("FindFirstChild() expected child but got nil")
				return
			}

			if child.GetTagName() != tt.wantTag {
				t.Errorf("FindFirstChild() tag = %v, want %v", child.GetTagName(), tt.wantTag)
			}
		})
	}
}

func TestMJMLNode_GetTextContent(t *testing.T) {
	input := `<mjml><mj-body><mj-text>  Hello World  </mj-text></mj-body></mjml>`
	node, err := ParseMJML(input)
	if err != nil {
		t.Fatalf("ParseMJML() error = %v", err)
	}

	textNode := node.FindFirstChild("mj-body").FindFirstChild("mj-text")
	if textNode == nil {
		t.Fatal("Could not find mj-text node")
	}

	got := textNode.GetTextContent()
	want := "Hello World"

	if got != want {
		t.Errorf("GetTextContent() = %q, want %q", got, want)
	}
}

// TestMJMLNode_HasRenderableMixedContent checks that the cheap presence
// check agrees with what GetMixedContent would actually produce: empty or
// whitespace-only content reports false, while any real text or nested
// element reports true.
func TestMJMLNode_HasRenderableMixedContent(t *testing.T) {
	//GIVEN: mj-text nodes covering empty, whitespace-only, text-only, and
	//child-only content.
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", `<mjml><mj-body><mj-text></mj-text></mj-body></mjml>`, false},
		{"whitespace only", `<mjml><mj-body><mj-text>   </mj-text></mj-body></mjml>`, false},
		{"plain text", `<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>`, true},
		{"child element only", `<mjml><mj-body><mj-text><b></b></mj-text></mj-body></mjml>`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//WHEN: the node is parsed and asked whether it has renderable
			//mixed content.
			node, err := ParseMJML(tt.input)
			if err != nil {
				t.Fatalf("ParseMJML() error = %v", err)
			}
			textNode := node.FindFirstChild("mj-body").FindFirstChild("mj-text")
			if textNode == nil {
				t.Fatal("Could not find mj-text node")
			}

			got := textNode.HasRenderableMixedContent()

			//THEN: the result matches whether GetMixedContent would have
			//returned non-empty content.
			if got != tt.want {
				t.Errorf("HasRenderableMixedContent() = %v, want %v", got, tt.want)
			}
			if want := textNode.GetMixedContent() != ""; got != want {
				t.Errorf("HasRenderableMixedContent() = %v disagrees with GetMixedContent() != \"\" = %v", got, want)
			}
		})
	}
}

func TestMJMLRaw_SingleVoidElement(t *testing.T) {
	mjml := `<mjml>
<mj-body>
	<mj-raw>
		<p>Text before void element</p>
		<img src="test.jpg" alt="test">
		<p>Text after void element</p>
	</mj-raw>
</mj-body>
</mjml>`

	node, err := ParseMJML(mjml)
	if err != nil {
		t.Fatalf("MJML with single void element should parse without error: %v", err)
	}

	rawElement := node.FindFirstChild("mj-body").FindFirstChild("mj-raw")
	if rawElement == nil {
		t.Fatal("Should find mj-raw element")
	}

	if !strings.Contains(rawElement.Text, "<img") {
		t.Error("Raw content should preserve img tag")
	}
}

func TestMJMLRaw_MultipleVoidElements(t *testing.T) {
	mjml := `<mjml>
<mj-body>
	<mj-raw>
		<p>Paragraph with <br> line break</p>
		<hr>
		<img src="test.jpg" alt="test">
		<input type="text" name="test">
		<p>End paragraph</p>
	</mj-raw>
</mj-body>
</mjml>`

	node, err := ParseMJML(mjml)
	if err != nil {
		t.Fatalf("MJML with multiple void elements should parse without error: %v", err)
	}

	rawElement := node.FindFirstChild("mj-body").FindFirstChild("mj-raw")
	if rawElement == nil {
		t.Fatal("Should find mj-raw element")
	}

	voidTags := []string{"<br", "<hr", "<img", "<input"}
	for _, tag := range voidTags {
		if !strings.Contains(rawElement.Text, tag) {
			t.Errorf("Raw content should preserve %s tag", tag)
		}
	}
}

func TestMJMLRaw_NestedVoidElements(t *testing.T) {
	mjml := `<mjml>
<mj-body>
	<mj-raw>
		<div>
			<p>Text <img src="icon.png" alt="icon"> with image</p>
			<p>Another <br> paragraph</p>
		</div>
	</mj-raw>
</mj-body>
</mjml>`

	node, err := ParseMJML(mjml)
	if err != nil {
		t.Fatalf("MJML with nested void elements should parse without error: %v", err)
	}

	rawElement := node.FindFirstChild("mj-body").FindFirstChild("mj-raw")
	if rawElement == nil {
		t.Fatal("Should find mj-raw element")
	}

	if !strings.Contains(rawElement.Text, "<div>") || !strings.Contains(rawElement.Text, "</div>") {
		t.Error("Raw content should preserve div nesting")
	}

	if !strings.Contains(rawElement.Text, "<img") {
		t.Error("Raw content should preserve img tag")
	}

	if !strings.Contains(rawElement.Text, "<br") {
		t.Error("Raw content should preserve br tag")
	}
}

func TestMJMLRaw_PreservesCompleteHTMLStructure(t *testing.T) {
	mjml := `<mjml>
<mj-body>
	<mj-raw>
		<div class="container">
			<img src="test.jpg" alt="test" />
			<p>Content after void element</p>
			<hr />
			<p>Final paragraph</p>
		</div>
	</mj-raw>
</mj-body>
</mjml>`

	node, err := ParseMJML(mjml)
	if err != nil {
		t.Fatalf("Parse should succeed: %v", err)
	}

	rawElement := node.FindFirstChild("mj-body").FindFirstChild("mj-raw")
	if rawElement == nil {
		t.Fatal("Should find mj-raw element")
	}

	content := rawElement.Text

	// The parser must preserve the complete HTML structure including all content after void elements
	requiredElements := []string{
		`<div class="container">`,
		`</div>`,
		`<img src="test.jpg" alt="test"`,
		`<p>Content after void element</p>`,
		`<hr`,
		`<p>Final paragraph</p>`,
	}

	t.Logf("Raw content: %q", content)

	for _, required := range requiredElements {
		if !strings.Contains(content, required) {
			t.Errorf("Raw content missing required element: %s\nActual content: %s", required, content)
		}
	}
}

func TestEscapeAttributeAmpersands(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with query parameters",
			input:    `<mj-image src="https://example.com?param1=value1&param2=value2" />`,
			expected: `<mj-image src="https://example.com?param1=value1&amp;param2=value2" />`,
		},
		{
			name:     "Multiple URL parameters",
			input:    `<mj-button href="https://example.com?a=1&b=2&c=3&d=4" />`,
			expected: `<mj-button href="https://example.com?a=1&amp;b=2&amp;c=3&amp;d=4" />`,
		},
		{
			name:     "Mixed valid entities and raw ampersands",
			input:    `<mj-text>Hello &amp; World</mj-text><mj-image src="https://example.com?test=1&param=2" />`,
			expected: `<mj-text>Hello &amp; World</mj-text><mj-image src="https://example.com?test=1&amp;param=2" />`,
		},
		{
			name:     "Single quotes in attributes",
			input:    `<mj-image src='https://example.com?param1=value1&param2=value2' />`,
			expected: `<mj-image src='https://example.com?param1=value1&amp;param2=value2' />`,
		},
		{
			name:     "An entity is escaped, so that the value stays as written",
			input:    `<mj-image src="https://example.com?param1=value1&amp;param2=value2" />`,
			expected: `<mj-image src="https://example.com?param1=value1&amp;amp;param2=value2" />`,
		},
		{
			name:     "Multiple attributes with ampersands",
			input:    `<mj-button href="https://example.com?a=1&b=2" title="Click & Win" />`,
			expected: `<mj-button href="https://example.com?a=1&amp;b=2" title="Click &amp; Win" />`,
		},
		{
			name:     "Named entities are kept as written",
			input:    `<mj-text content="&lt;b&gt;Bold&lt;/b&gt; &amp; &copy; 2024" />`,
			expected: `<mj-text content="&amp;lt;b&amp;gt;Bold&amp;lt;/b&amp;gt; &amp;amp; &amp;copy; 2024" />`,
		},
		{
			name:     "Numeric character references",
			input:    `<mj-text content="Price: &#36;100 &amp; tax &#160;included" />`,
			expected: `<mj-text content="Price: &amp;#36;100 &amp;amp; tax &amp;#160;included" />`,
		},
		{
			name:     "Hexadecimal character references",
			input:    `<mj-text content="Unicode: &#x20AC; &amp; more" />`,
			expected: `<mj-text content="Unicode: &amp;#x20AC; &amp;amp; more" />`,
		},
		{
			name:     "Complex URL with fragment and special characters",
			input:    `<mj-button href="https://example.com/path?query=test&filter=active#section&more" />`,
			expected: `<mj-button href="https://example.com/path?query=test&amp;filter=active#section&amp;more" />`,
		},
		{
			name:     "Multiple components on same line",
			input:    `<mj-image src="https://img.com?w=300&h=200" /><mj-button href="https://btn.com?action=click&ref=email" />`,
			expected: `<mj-image src="https://img.com?w=300&amp;h=200" /><mj-button href="https://btn.com?action=click&amp;ref=email" />`,
		},
		{
			name:     "Attributes without quotes should be ignored",
			input:    `<mj-image src=https://example.com?param1=value1&param2=value2 width="300px" />`,
			expected: `<mj-image src=https://example.com?param1=value1&param2=value2 width="300px" />`,
		},
		{
			name:     "Empty attributes",
			input:    `<mj-image src="" alt="" />`,
			expected: `<mj-image src="" alt="" />`,
		},
		{
			name:     "Attributes with spaces around equals",
			input:    `<mj-image src = "https://example.com?a=1&b=2" width= "300px" />`,
			expected: `<mj-image src = "https://example.com?a=1&amp;b=2" width= "300px" />`,
		},
		{
			name:     "No attributes to process",
			input:    `<mj-text>Just some content with & ampersand</mj-text>`,
			expected: `<mj-text>Just some content with & ampersand</mj-text>`,
		},
		{
			name:     "Social media URLs",
			input:    `<mj-social-element href="https://www.facebook.com/sharer/sharer.php?u=https://example.com&t=My Title" />`,
			expected: `<mj-social-element href="https://www.facebook.com/sharer/sharer.php?u=https://example.com&amp;t=My Title" />`,
		},
		{
			name:     "Carousel images with thumbnails",
			input:    `<mj-carousel-image src="https://img.com/main.jpg" thumbnails-src="https://img.com/thumb.jpg?w=100&h=100" />`,
			expected: `<mj-carousel-image src="https://img.com/main.jpg" thumbnails-src="https://img.com/thumb.jpg?w=100&amp;h=100" />`,
		},
		{
			name:     "Navbar with base URL",
			input:    `<mj-navbar base-url="https://example.com" href="/path?param=value&other=test" />`,
			expected: `<mj-navbar base-url="https://example.com" href="/path?param=value&amp;other=test" />`,
		},
		{
			name:     "Hero background URL",
			input:    `<mj-hero background-url="https://images.com/hero.jpg?quality=high&format=webp" />`,
			expected: `<mj-hero background-url="https://images.com/hero.jpg?quality=high&amp;format=webp" />`,
		},
		{
			name:     "Accordion icon URLs",
			input:    `<mj-accordion icon-wrapped-url="https://icons.com/plus.svg?color=blue&size=16" icon-unwrapped-url="https://icons.com/minus.svg?color=red&size=16" />`,
			expected: `<mj-accordion icon-wrapped-url="https://icons.com/plus.svg?color=blue&amp;size=16" icon-unwrapped-url="https://icons.com/minus.svg?color=red&amp;size=16" />`,
		},
		{
			name:     "The original failing case",
			input:    `<mj-image width="300px" src="https://placehold.co/150x60/E3F2FD/1976D2?font=playfair-display&text=LOGO" />`,
			expected: `<mj-image width="300px" src="https://placehold.co/150x60/E3F2FD/1976D2?font=playfair-display&amp;text=LOGO" />`,
		},
		{
			// A backslash escapes nothing: the value ends at the second ",
			// and the next quote, not after an =, opens no value (as
			// htmlparser2 reads it), so the & is outside any value.
			name:     "Edge case: Attribute value contains quotes",
			input:    `<mj-text title="Say \"Hello\" & welcome" />`,
			expected: `<mj-text title="Say \"Hello\" & welcome" />`,
		},
		{
			name:     "Edge case: Mixed quote types",
			input:    `<mj-button href='https://example.com?msg=Hello&name=World' title="Click & Go" />`,
			expected: `<mj-button href='https://example.com?msg=Hello&amp;name=World' title="Click &amp; Go" />`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeAttributeAmpersands(tt.input)
			if result != tt.expected {
				t.Errorf("escapeAttributeAmpersands() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPreprocessHTMLEntitiesIntegration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Full MJML with URL parameters",
			input: `<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-image width="300px" src="https://placehold.co/150x60/E3F2FD/1976D2?font=playfair-display&text=LOGO" />
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`,
			expected: `<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-image width="300px" src="https://placehold.co/150x60/E3F2FD/1976D2?font=playfair-display&amp;text=LOGO" />
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`,
		},
		{
			name: "Complex MJML with multiple components and entities",
			input: `<mjml>
  <mj-body>
    <mj-text>&copy; 2024 Company &amp; Co.</mj-text>
    <mj-button href="https://example.com?utm_source=email&utm_medium=newsletter">Visit &amp; Buy</mj-button>
    <mj-image src="https://images.com/logo.png?w=300&h=150&quality=high" alt="Logo &amp; Tagline" />
  </mj-body>
</mjml>`,
			expected: `<mjml>
  <mj-body>
    <mj-text>© 2024 Company &amp; Co.</mj-text>
    <mj-button href="https://example.com?utm_source=email&amp;utm_medium=newsletter">Visit &amp; Buy</mj-button>
    <mj-image src="https://images.com/logo.png?w=300&amp;h=150&amp;quality=high" alt="Logo &amp;amp; Tagline" />
  </mj-body>
</mjml>`,
		},
		{
			name: "Social component URLs",
			input: `<mjml>
  <mj-body>
    <mj-social>
      <mj-social-element name="facebook" href="https://facebook.com/share?u=https://example.com&t=Check this out" src="https://icons.com/fb.svg?size=32&color=blue" />
      <mj-social-element name="twitter" href="https://twitter.com/intent/tweet?text=Hello&url=https://example.com" />
    </mj-social>
  </mj-body>
</mjml>`,
			expected: `<mjml>
  <mj-body>
    <mj-social>
      <mj-social-element name="facebook" href="https://facebook.com/share?u=https://example.com&amp;t=Check this out" src="https://icons.com/fb.svg?size=32&amp;color=blue" />
      <mj-social-element name="twitter" href="https://twitter.com/intent/tweet?text=Hello&amp;url=https://example.com" />
    </mj-social>
  </mj-body>
</mjml>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessHTMLEntities(tt.input)
			if result != tt.expected {
				t.Errorf("preprocessHTMLEntities() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStripNonMSOComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    "<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
			expected: "<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
		},
		{
			name:     "regular comment should be stripped",
			input:    "<!-- This is a regular comment --><mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
			expected: "<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
		},
		{
			name:     "AIDEV comment should be stripped",
			input:    "<!-- AIDEV-NOTE: test comment --><mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
			expected: "<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
		},
		{
			name:     "multiple regular comments should be stripped",
			input:    "<!-- Comment 1 --><!-- Comment 2 --><mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
			expected: "<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
		},
		{
			name:     "MSO conditional comment before root is stripped",
			input:    "<!--[if mso]><xml>MSO content</xml><![endif]--><mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
			expected: "<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
		},
		{
			name:     "mixed comments before root are stripped",
			input:    "<!-- Regular comment --><!--[if mso]><xml>MSO</xml><![endif]--><!-- Another comment --><mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
			expected: "<mjml><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
		},
		{
			name:     "comment inside mjml should be preserved",
			input:    "<mjml><!-- inside --><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
			expected: "<mjml><!-- inside --><mj-body><mj-text>Hello</mj-text></mj-body></mjml>",
		},
		{
			name:  "real world case - AIDEV comments before mjml",
			input: "<!-- AIDEV-NOTE: width-calculation-test; tests wrapper padding reducing section max-width -->\n<!-- Wrapper padding=\"20px\" should reduce child section containerWidth from 600px to 560px -->\n<mjml>\n  <mj-body background-color=\"#f8f9fa\">\n  </mj-body>\n</mjml>",
			// The comments go, their line breaks stay: warnings report the
			// template's lines.
			expected: "\n\n<mjml>\n  <mj-body background-color=\"#f8f9fa\">\n  </mj-body>\n</mjml>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripNonMSOComments(tt.input)
			if result != tt.expected {
				t.Errorf("stripNonMSOComments() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseMJMLWithComments(t *testing.T) {
	// Test the actual failing case
	input := `<!-- AIDEV-NOTE: width-calculation-test; tests wrapper padding reducing section max-width -->
<!-- Wrapper padding="20px" should reduce child section containerWidth from 600px to 560px -->
<mjml>
  <mj-body background-color="#f8f9fa">
    <mj-wrapper padding="20px">
      <mj-section full-width="full-width" padding="80px 20px">
        <mj-column width="100%" padding="0">
          <mj-text>Background Image Section</mj-text>
        </mj-column>
      </mj-section>
    </mj-wrapper>
  </mj-body>
</mjml>`

	// Debug: check what the stripped content looks like
	stripped := stripNonMSOComments(input)
	t.Logf("Stripped content: %q", stripped)

	root, err := ParseMJML(input)
	if err != nil {
		t.Fatalf("ParseMJML failed: %v", err)
	}

	if root.GetTagName() != "mjml" {
		t.Errorf("Expected root tag 'mjml', got '%s'", root.GetTagName())
	}

	// Verify the structure is correct
	body := root.FindFirstChild("mj-body")
	if body == nil {
		t.Error("Expected to find mj-body child")
	}

	if body != nil {
		wrapper := body.FindFirstChild("mj-wrapper")
		if wrapper == nil {
			t.Error("Expected to find mj-wrapper child")
		}
	}
}

func TestDecodeXMLEntitiesOutsideAttributes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "content without entities is returned unchanged",
			input:    `<mj-text>plain text</mj-text>`,
			expected: `<mj-text>plain text</mj-text>`,
		},
		{
			// &lt; and &gt; stay: decoded before the XML parse, escaped text
			// would become markup (an mj-button from escaped data).
			name:     "quote entities in element text are decoded, angle brackets are not",
			input:    `<mj-text>&lt;b&gt;bold&lt;/b&gt; &quot;quoted&quot; it&apos;s</mj-text>`,
			expected: `<mj-text>&lt;b&gt;bold&lt;/b&gt; "quoted" it's</mj-text>`,
		},
		{
			name:     "double-quoted attribute value is left for the XML parser",
			input:    `<mj-image alt="27&quot; monitor &lt;b&gt;" />`,
			expected: `<mj-image alt="27&quot; monitor &lt;b&gt;" />`,
		},
		{
			name:     "single-quoted attribute value is left for the XML parser",
			input:    `<mj-image alt='it&apos;s &quot;new&quot;' />`,
			expected: `<mj-image alt='it&apos;s &quot;new&quot;' />`,
		},
		{
			name:     "angle bracket inside a quoted value does not end the tag",
			input:    `<mj-image title="1 > 0 &gt; -1" alt='&lt;' />&lt;`,
			expected: `<mj-image title="1 > 0 &gt; -1" alt='&lt;' />&lt;`,
		},
		{
			name:     "quotes in element text do not open an attribute value",
			input:    `<mj-text>say "&lt;hi&gt;"</mj-text>`,
			expected: `<mj-text>say "&lt;hi&gt;"</mj-text>`,
		},
		{
			name:     "ampersand, unknown and unterminated entities are preserved",
			input:    `<mj-text>&amp;lt; &copy; &lt</mj-text>`,
			expected: `<mj-text>&amp;lt; &copy; &lt</mj-text>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeXMLEntitiesOutsideAttributes(tt.input); got != tt.expected {
				t.Errorf("decodeXMLEntitiesOutsideAttributes() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseMJMLKeepsAttributeValuesAsWritten(t *testing.T) {
	// As MJML (htmlparser2 with decodeEntities: false): an attribute value
	// keeps its entities, which the HTML output writes as they are.
	node, err := ParseMJML(`<mjml><mj-body><mj-image alt="27&quot; monitor &lt;b&gt; &amp;copy;" title='it&apos;s "a" <b>' /></mj-body></mjml>`)
	if err != nil {
		t.Fatalf("ParseMJML() error = %v", err)
	}
	if len(node.Children) != 1 || len(node.Children[0].Children) != 1 {
		t.Fatalf("ParseMJML() returned an unexpected tree: %+v", node)
	}

	image := node.Children[0].Children[0]
	if got, want := image.GetAttribute("alt"), `27&quot; monitor &lt;b&gt; &amp;copy;`; got != want {
		t.Errorf("alt = %q, want %q", got, want)
	}
	if got, want := image.GetAttribute("title"), `it&apos;s "a" <b>`; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

// A < in a quoted value is kept as written even when the document holds no
// &, which used to skip the escaping before the XML parse (and fail it).
func TestParseMJMLKeepsAngleBracketsWithoutAmpersand(t *testing.T) {
	node, err := ParseMJML(`<mjml><mj-body><mj-image src="x.png" alt="a<b" title='1 < 2 > 0' /></mj-body></mjml>`)
	if err != nil {
		t.Fatalf("ParseMJML() error = %v", err)
	}
	image := node.Children[0].Children[0]
	if got := image.GetAttribute("alt"); got != "a<b" {
		t.Errorf("alt = %q, want %q", got, "a<b")
	}
	if got := image.GetAttribute("title"); got != "1 < 2 > 0" {
		t.Errorf("title = %q, want %q", got, "1 < 2 > 0")
	}
	if got := escapeAttributeAmpersands(`<a b="c">d</a>`); got != `<a b="c">d</a>` {
		t.Errorf("a template with nothing to escape changed: %q", got)
	}
}

// A comment with "--" in it, which the XML decoder refuses, is read as
// htmlparser2 reads it, up to its first "-->", and kept as written, the
// private-use characters of the encoding included.
func TestCommentWithDashes(t *testing.T) {
	for _, comment := range []string{
		" ---------- HEADER ---------- ",
		" a -",
		"----",
		" --  -- ",
		"  ",
	} {
		node, err := ParseMJML("<mjml><mj-body><mj-raw>x</mj-raw><!--" + comment + "--></mj-body></mjml>")
		if err != nil {
			t.Errorf("%q: %v", comment, err)
			continue
		}
		body := node.Children[0]
		if want := "<!--" + comment + "-->"; !strings.Contains(body.Text, want) {
			t.Errorf("%q: body text %q, want %q in it", comment, body.Text, want)
		}
	}
}

// A comment ends at the first "-->" counted from the dashes of its "<!--",
// as htmlparser2 reads it: "<!-->" and "<!--->" are empty comments.
func TestCommentEnd(t *testing.T) {
	for _, c := range []struct {
		s    string
		want int
	}{
		{"<!-->x", 5},
		{"<!--->x", 6},
		{"<!---->x", 7},
		{"<!--x-->", 8},
		{"<!--x--->", 9},
		{"<!-- a -> b -->", 15},
		{"<!--x->", -1},
		{"<!--", -1},
		{"<!-", -1},
	} {
		if got := CommentEnd(c.s, 0); got != c.want {
			t.Errorf("CommentEnd(%q) = %d, want %d", c.s, got, c.want)
		}
		if got := CommentEnd([]byte(c.s), 0); got != c.want {
			t.Errorf("CommentEnd([]byte(%q)) = %d, want %d", c.s, got, c.want)
		}
	}
}
