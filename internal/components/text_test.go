package components

import (
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestMJTextRenderHTMLAsIs tests that mj-text renders HTML content as-is without parsing or correction.
// This test guards the spec requirement that mj-text is an "ending tag" component where all content
// is passed through directly to the output HTML without any additional MJML parsing, escaping, or correction.
func TestMJTextRenderHTMLAsIs(t *testing.T) {
	testCases := []struct {
		name     string
		mjml     string
		expected string
		desc     string
	}{
		{
			name:     "Standard unclosed br tag",
			mjml:     `<mj-text>Hello<br>World!</mj-text>`,
			expected: "Hello<br>World!",
			desc:     "Unclosed <br> tags should be preserved as-is",
		},
		{
			name:     "Self-closing br tag",
			mjml:     `<mj-text>Hello<br/>World!</mj-text>`,
			expected: "Hello<br/>World!",
			desc:     "Self-closing <br/> tags are kept as written, as MJML keeps ending-tag content",
		},
		{
			name:     "Self-closing br with space",
			mjml:     `<mj-text>Hello<br />World!</mj-text>`,
			expected: "Hello<br />World!",
			desc:     "Self-closing <br /> tags with space should be preserved as-is",
		},
		{
			name:     "Malformed br tag",
			mjml:     `<mj-text>Hello<br bar>World!</mj-text>`,
			expected: "Hello<br bar>World!",
			desc:     "Malformed HTML tags should be passed through unchanged",
		},
		{
			name:     "Mixed br formats",
			mjml:     `<mj-text>Line1<br>Line2<br/>Line3<br />Line4</mj-text>`,
			expected: "Line1<br>Line2<br/>Line3<br />Line4",
			desc:     "Mixed formats, each kept as written",
		},
		{
			name:     "Complex HTML tags",
			mjml:     `<mj-text>Hello<span style="color: red">colored</span> text</mj-text>`,
			expected: "Hello<span style=\"color: red\">colored</span> text",
			desc:     "Complex HTML tags should be preserved with attributes",
		},
		{
			name:     "Unclosed HTML tag",
			mjml:     `<mj-text>Hello<strong>bold text</mj-text>`,
			expected: "Hello<strong>bold text",
			desc:     "Unclosed HTML tags should be passed through as-is",
		},
		{
			name:     "Unclosed img tag",
			mjml:     `<mj-text>Check this <img src="test.jpg"> image</mj-text>`,
			expected: "Check this <img src=\"test.jpg\"> image",
			desc:     "Unclosed <img> tags should be preserved as-is",
		},
		{
			name:     "Self-closing img tag",
			mjml:     `<mj-text>Check this <img src="test.jpg"/> image</mj-text>`,
			expected: "Check this <img src=\"test.jpg\"/> image",
			desc:     "Self-closing <img/> tags are kept as written",
		},
		{
			name:     "Self-closing img with space",
			mjml:     `<mj-text>Check this <img src="test.jpg" /> image</mj-text>`,
			expected: "Check this <img src=\"test.jpg\" /> image",
			desc:     "Self-closing <img /> tags with space should be preserved as-is",
		},
		{
			name:     "Mixed void elements",
			mjml:     `<mj-text>Line1<br>Image: <img src="test.jpg"><br/>End</mj-text>`,
			expected: "Line1<br>Image: <img src=\"test.jpg\"><br/>End",
			desc:     "Mixed void elements, each kept as written",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the MJML
			ast, err := parser.ParseMJML(tc.mjml)
			if err != nil {
				t.Fatalf("Failed to parse MJML: %v", err)
			}

			// Find the mj-text node
			var textNode *parser.MJMLNode
			findTextNode(ast, &textNode)
			if textNode == nil {
				t.Fatal("Could not find mj-text node in parsed AST")
			}

			// Create the component
			component := NewMJTextComponent(textNode, &options.RenderOpts{})

			// Test the actual rendered inner HTML content
			output, err := component.buildRawInnerHTML()
			if err != nil {
				t.Fatalf("Failed to build raw inner HTML: %v", err)
			}
			actual := strings.TrimSpace(output)

			if actual != tc.expected {
				t.Errorf("%s\nExpected: %q\nActual:   %q", tc.desc, tc.expected, actual)

				// Additional debugging information
				t.Logf("Raw node text: %q", textNode.Text)
				t.Logf("Mixed content parts: %d", len(textNode.MixedContent))
				for i, part := range textNode.MixedContent {
					if part.Node != nil {
						t.Logf("Part[%d]: Node <%s>", i, part.Node.XMLName.Local)
					} else {
						t.Logf("Part[%d]: Text %q", i, part.Text)
					}
				}
			}
		})
	}
}

// TestMJTextWithoutCloseTag checks an mj-text MJML never sees closed: as in
// mjml-parser-xml, its content runs to the end of the document and is kept
// up to the last "</mj-text", here none, so it is empty.
func TestMJTextWithoutCloseTag(t *testing.T) {
	node, err := parser.ParseMJML(`<mj-text>Hello<br>World!`)
	if err != nil {
		t.Fatal(err)
	}
	if node.Text != "" || len(node.Children) != 0 {
		t.Errorf("content %q, %d children, want none", node.Text, len(node.Children))
	}
}
