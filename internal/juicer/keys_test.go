package juicer

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// TestRarestClassKeyChangesNothing inlines random stylesheets whose rules
// name several classes in their rightmost compound, keyed by the rarest
// class of the document and by the first, and requires the same bytes: the
// key only chooses which elements test a rule.
func TestRarestClassKeyChangesNothing(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	classes := []string{"a", "b", "c", "d"}
	tags := []string{"p", "div", "span", "td"}
	compound := func() string {
		var b strings.Builder
		if r.Intn(3) == 0 {
			b.WriteString(tags[r.Intn(len(tags))])
		}
		for range 1 + r.Intn(3) {
			b.WriteString("." + classes[r.Intn(len(classes))])
		}
		if r.Intn(4) == 0 {
			b.WriteString(":first-child")
		}
		return b.String()
	}
	attr := func() string {
		var cs []string
		for range r.Intn(4) {
			cs = append(cs, classes[r.Intn(len(classes))])
		}
		return strings.Join(cs, " ")
	}
	const cases = 2000
	rekeyed := 0
	for n := range cases {
		var css strings.Builder
		for range 2 + r.Intn(8) {
			sel := compound()
			if r.Intn(3) == 0 {
				sel = compound() + []string{" ", " > ", " + "}[r.Intn(3)] + sel
			}
			fmt.Fprintf(&css, "%s{color:#%03x;margin:%dpx}", sel, r.Intn(4096), r.Intn(9))
		}
		var body strings.Builder
		for range 3 + r.Intn(10) {
			tag := tags[r.Intn(len(tags))]
			fmt.Fprintf(&body, `<%s class="%s"><span class="%s">x</span></%s>`, tag, attr(), attr(), tag)
		}
		doc := "<style>" + css.String() + "</style><div>" + body.String() + "</div>"
		in := mustNew(t)
		rarest, errRarest := in.Inline(doc)
		rarestClassKey = false
		first, errFirst := in.Inline(doc)
		rarestClassKey = true
		if rarest != first || (errRarest == nil) != (errFirst == nil) {
			t.Fatalf("case %d: the key changes the output\ncss: %s\nrarest: %s\nfirst:  %s", n, css.String(), rarest, first)
		}
		if strings.Contains(css.String(), ".a.b") || strings.Contains(css.String(), ".b.a") {
			rekeyed++
		}
	}
	if rekeyed < cases/4 {
		t.Errorf("only %d of %d stylesheets have a rule with two classes", rekeyed, cases)
	}
}
