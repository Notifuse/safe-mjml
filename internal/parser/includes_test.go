package parser

import "testing"

// TestParseDropsIncludes checks that the parser removes mj-include, nested
// ones too, and puts what each holds in its place, as MJML 5's parser does
// (ignoreIncludes): the elements become children of the parent, in order,
// its text the parent's text, and the removed elements are kept in
// Includes for the warning.
func TestParseDropsIncludes(t *testing.T) {
	root, err := ParseMJML(`<mjml><mj-body><mj-section>` +
		`<mj-column/>` +
		`<mj-include path="a"><mj-column/><mj-include path="b">x<mj-column/></mj-include></mj-include>` +
		`<mj-include path="c"/>` +
		`<mj-column/>` +
		`</mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	section := root.Children[0].Children[0]
	if got := len(section.Children); got != 4 {
		t.Fatalf("children: got %d, want the 4 columns", got)
	}
	for i, child := range section.Children {
		if child.GetTagName() != "mj-column" {
			t.Errorf("child %d: got %s, want mj-column", i, child.GetTagName())
		}
	}
	if section.Text != "x" {
		t.Errorf("text: got %q, want the text of the nested include, %q", section.Text, "x")
	}
	var paths []string
	for _, include := range section.Includes {
		for _, a := range include.Attrs {
			paths = append(paths, a.Value)
		}
	}
	if len(paths) != 3 || paths[0] != "a" || paths[1] != "b" || paths[2] != "c" {
		t.Errorf("includes: got %q, want [a b c]", paths)
	}
	var nodes int
	for _, part := range section.MixedContent {
		if part.Node != nil {
			if part.Node.GetTagName() == "mj-include" {
				t.Error("an mj-include is left in the mixed content")
			}
			nodes++
		}
	}
	if nodes != 4 {
		t.Errorf("mixed content: got %d elements, want 4", nodes)
	}
}
