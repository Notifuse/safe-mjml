package juicer

import "testing"

// juice11Inline inlines css into doc as MJML 5.4.1 calls juice 11.1.1:
// juice(html, {applyStyleTags: false, extraCss, insertPreservedExtraCss:
// false, removeStyleTags: false}).
func juice11Inline(t *testing.T, doc, css string) string {
	t.Helper()
	out, err := Inline(doc, CompatibilityMode(Juice11), ExtraCSS(css), ApplyStyleTags(false), RemoveStyleTags(false), InsertPreservedExtraCSS(false))
	if err != nil {
		t.Fatalf("Inline(%q) with %q: %v", doc, css, err)
	}
	return out
}

// TestJuice11 checks the Juice11 profile against juice 11.1.1 (the outputs
// below are its own, from tools/mjml-reference/node_modules/juice): its
// ignore comments, first in the stylesheet or not, "start ignore" with and
// without its end, "ignore next" before a rule (which also consumes a
// comment after it), a declaration, the last declaration, and in a style
// attribute; two inline mj-style blocks joined, where "juice ignore" is not
// first; a rule without declarations, which no longer rewrites the style
// attribute; and a textarea, whose content is raw text.
func TestJuice11(t *testing.T) {
	tests := []struct{ name, doc, css, want string }{
		{"ignore-first", `<p>x</p>`, `/* juice ignore */ p{color:red}`, `<p>x</p>`},
		{"ignore-not-first", `<p>x</p><b>y</b>`, `p{color:red} /* juice ignore */ b{color:blue}`, `<p style="color: red;">x</p><b style="color: blue;">y</b>`},
		{"start-end", `<p>x</p><b>y</b>`, `/* juice start ignore */ p{color:red} /* juice end ignore */ b{color:blue}`, `<p>x</p><b style="color: blue;">y</b>`},
		{"start-no-end", `<p>x</p><b>y</b>`, `b{color:blue} /* juice start ignore */ p{color:red}`, `<p>x</p><b style="color: blue;">y</b>`},
		{"next-rule-then-comment", `<p>x</p><b>y</b>`, `/* juice ignore next */ /* c */ p{color:red} b{color:blue}`, `<p style="color: red;">x</p><b style="color: blue;">y</b>`},
		{"next-rule", `<p>x</p><b>y</b>`, `/* juice ignore next */ p{color:red} b{color:blue}`, `<p>x</p><b style="color: blue;">y</b>`},
		{"next-decl", `<p>x</p>`, `p{ /* juice ignore next */ color:red; font-weight:bold }`, `<p style="font-weight: bold;">x</p>`},
		{"next-last-decl", `<p>x</p>`, `p{ color:red; /* juice ignore next */ font-weight:bold }`, `<p style="color: red;">x</p>`},
		{"next-in-style", `<p style="/* juice ignore next */ color:green; margin:0">x</p>`, `p{font-weight:bold}`, `<p style="font-weight: bold; margin: 0;">x</p>`},
		{"joined-blocks", `<p>x</p><b>y</b>`, `p{color:red}/* juice ignore */ b{color:blue}`, `<p style="color: red;">x</p><b style="color: blue;">y</b>`},
		{"empty-rule", `<p style="color:red">x</p>`, `p{}`, `<p style="color:red">x</p>`},
		{"comment-only-rule", `<p style="color:red">x</p>`, `p{/* c */}`, `<p style="color:red">x</p>`},
		{"textarea", `<textarea><p>x</p></textarea><p>y</p>`, `p{color:red}`, `<textarea><p>x</p></textarea><p style="color: red;">y</p>`},
		{"textarea-upper", `<TEXTAREA><p>x</p></TEXTAREA ><p>y</p>`, `p{color:red}`, `<textarea><p>x</p></textarea><p style="color: red;">y</p>`},
		{"textarea-markup", `<textarea><b>x</b></textarea><b>y</b>`, `b{color:red}`, `<textarea><b>x</b></textarea><b style="color: red;">y</b>`},
		{"textarea-self-closed", `<textarea/><b>x</b></textarea><b>y</b>`, `b{color:red}`, `<textarea><b style="color: red;">x</b></textarea><b style="color: red;">y</b>`},
		{"textarea-close-with-attribute", `<textarea><b>x</textarea x><b>y</b>`, `b{color:red}`, `<textarea><b>x</textarea><b style="color: red;">y</b>`},
		{"textarea-longer-close", `<textarea>a</textareax><b>b</b></textarea>c`, `b{color:red}`, `<textarea>a</textareax><b>b</b></textarea>c`},
		{"textarea-unclosed", `<textarea><b>x`, `b{color:red}`, `<textarea><b>x</textarea>`},
		{"textarea-mixed-case", "<TextArea><b>c</b></TEXTAREA\t><b>d</b>", `b{color:red}`, `<textarea><b>c</b></textarea><b style="color: red;">d</b>`},
		{"textarea-lt-before-close", `<textarea>a<</textarea><b>b</b>`, `b{color:red}`, `<textarea>a<</textarea><b style="color: red;">b</b>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := juice11Inline(t, tt.doc, tt.css); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

// TestJuice11StyleWithoutDeclarations checks a style attribute with no
// declaration on an element a rule matches: juice 11.1.1 throws there
// ("Cannot read properties of undefined (reading '1')": parseCSS returns no
// rule for it), where juice 10 renders. The Juice11 profile renders it as
// Juice10 does (Pierre's decision, 2026-11-17): the outputs below are juice
// 10.0.1's, but for the template block, which juicer keeps where juice drops
// it (Pierre's earlier decision on template blocks in style attributes).
func TestJuice11StyleWithoutDeclarations(t *testing.T) {
	tests := []struct{ style, want string }{
		{"{{ x }}", `<p style="font-weight: bold; {{ x }};">x</p>`},
		{";", `<p style="font-weight: bold;">x</p>`},
		{"foo", `<p style="font-weight: bold;">x</p>`},
		{" ", `<p style="font-weight: bold;">x</p>`},
		{"color:", `<p style="font-weight: bold;">x</p>`},
		{"} p {", `<p style="font-weight: bold;">x</p>`},
		// juice 11 throws; juice 10 reads the declaration the comment
		// would leave out.
		{"/* juice ignore next */ color:red", `<p style="font-weight: bold; color: red;">x</p>`},
	}
	for _, tt := range tests {
		doc := `<p style="` + tt.style + `">x</p>`
		if got := juice11Inline(t, doc, "p{font-weight:bold}"); got != tt.want {
			t.Errorf("style %q: got %s, want %s", tt.style, got, tt.want)
		}
		out, err := Inline(doc, CompatibilityMode(Juice10), ExtraCSS("p{font-weight:bold}"), ApplyStyleTags(false), RemoveStyleTags(false), InsertPreservedExtraCSS(false))
		if err != nil || out != tt.want {
			t.Errorf("style %q with Juice10: got %s, %v, want %s", tt.style, out, err, tt.want)
		}
	}
}

// TestJuice11StyleLaterRule checks the style attributes juice 11 reads
// from a later rule, because parseCSS drops the first one, left with no
// declaration after a "}" or an ignore comment (juice 11.1.1's outputs).
func TestJuice11StyleLaterRule(t *testing.T) {
	tests := []struct{ style, want string }{
		{"} p { margin:0", `<p style="font-weight: bold; margin: 0;">x</p>`},
		{"} /* juice ignore next */ p { margin:0 } q { padding:1px", `<p style="font-weight: bold; padding: 1px;">x</p>`},
		{"/* juice ignore next */ color:red; margin:0", `<p style="font-weight: bold; margin: 0;">x</p>`},
	}
	for _, tt := range tests {
		doc := `<p style="` + tt.style + `">x</p>`
		if got := juice11Inline(t, doc, "p{font-weight:bold}"); got != tt.want {
			t.Errorf("style %q: got %s, want %s", tt.style, got, tt.want)
		}
	}
}

// TestJuice10HasNoIgnoreComments checks that the Juice10 profile is
// unchanged: juice 10.0.1 has no ignore comments, rewrites the style
// attribute for an empty rule, and parses a textarea's content (its outputs).
func TestJuice10HasNoIgnoreComments(t *testing.T) {
	tests := []struct{ doc, css, want string }{
		{`<p>x</p>`, `/* juice ignore */ p{color:red}`, `<p style="color: red;">x</p>`},
		{`<p style="color:red">x</p>`, `p{}`, `<p style="color: red;">x</p>`},
		{`<textarea><p>x</p></textarea><p>y</p>`, `p{color:red}`, `<textarea><p style="color: red;">x</p></textarea><p style="color: red;">y</p>`},
	}
	for _, tt := range tests {
		out, err := Inline(tt.doc, CompatibilityMode(Juice10), ExtraCSS(tt.css), ApplyStyleTags(false), RemoveStyleTags(false), InsertPreservedExtraCSS(false))
		if err != nil || out != tt.want {
			t.Errorf("%s with %s: got %s, %v, want %s", tt.doc, tt.css, out, err, tt.want)
		}
	}
}
