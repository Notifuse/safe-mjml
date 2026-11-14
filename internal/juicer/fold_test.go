package juicer

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// TestFoldRepeatedSelectorsChangesNothing inlines random stylesheets that
// repeat selectors, with and without mergeRepeatedSelectors, and requires
// the same output byte for byte: !important in an earlier copy, a property
// declared twice in one rule, a more specific selector in between, shorthands
// and longhands, every option set that keeps folding on, and both juice
// profiles.
func TestFoldRepeatedSelectorsChangesNothing(t *testing.T) {
	selectors := []string{".a", ".b", "p", "p.a", "#x", ".a.b", "td .a", ".a:first-child", "[class~=a]", ":is(.a, .b)", "td", "td.a"}
	// The properties juice turns into attributes of table cells
	// (bgcolor, align, valign, width, height), whose order is that of the
	// properties.
	props := []string{"color", "padding", "padding-top", "margin", "font-size", "background", "width",
		"background-color", "text-align", "vertical-align", "height"}
	values := []string{"red", "blue", "1px", "2px 3px", "#fff", "linear-gradient(red, blue)", "10px !important", "0 !important", "right", "top"}
	body := `<table><tr><td class="a"><p class="a" id="x" style="color: green;">one</p><p class="a b">two</p><span class="b">three</span></td>` +
		`<td class="b a" style="text-align: left;">four</td></tr></table>`
	r := rand.New(rand.NewSource(7))
	modes := []struct {
		name string
		opts []Option
	}{
		{"default", nil},
		{"juice 9", []Option{CompatibilityMode(Juice9)}},
		{"juice 12", []Option{CompatibilityMode(Juice12)}},
		{"juice 10", []Option{CompatibilityMode(Juice10)}},
		{"preserve important", []Option{PreserveImportant(true)}},
	}
	folds := 0
	const cases = 3000
	for n := range cases {
		// Most stylesheets declare a property once per rule, which is what
		// lets a repeated selector fold; the others check that it does not.
		unique := r.Intn(10) < 7
		var css strings.Builder
		for range 2 + r.Intn(10) {
			fmt.Fprintf(&css, "%s{", selectors[r.Intn(len(selectors))])
			used := map[string]bool{}
			for range 1 + r.Intn(3) {
				p := props[r.Intn(len(props))]
				if unique && used[p] {
					continue
				}
				used[p] = true
				fmt.Fprintf(&css, "%s:%s;", p, values[r.Intn(len(values))])
			}
			css.WriteString("}")
		}
		doc := "<style>" + css.String() + "</style>" + body
		rules, _, _ := parseWithDirectives([]byte(css.String()), &mustNew(t).opts, 0, &ignoreState{})
		if folded := mergeRepeatedSelectors(rules); len(rules) > 0 && &folded[0] != &rules[0] {
			folds++
		}
		for _, m := range modes {
			in := mustNew(t, m.opts...)
			folded, errFolded := in.Inline(doc)
			foldRepeated = false
			plain, errPlain := in.Inline(doc)
			foldRepeated = true
			if folded != plain || (errFolded == nil) != (errPlain == nil) {
				t.Fatalf("case %d, %s: folding changes the output\ncss: %s\nfolded: %s (%v)\nplain:  %s (%v)", n, m.name, css.String(), folded, errFolded, plain, errPlain)
			}
		}
	}
	// The test means something only if folding happens often.
	if folds < cases/4 {
		t.Errorf("only %d of %d stylesheets folded a selector", folds, cases)
	}
	t.Logf("%d of %d stylesheets folded a selector", folds, cases)
}
