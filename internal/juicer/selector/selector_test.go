package selector

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCompile(t *testing.T) {
	juice := Mode{JuiceSpecificity: true}
	for _, c := range []struct {
		sel     string
		m       Mode
		want    string // spec, pseudo-element, then each variant as text|key|name
		ignored bool
	}{
		{sel: ":is(.b, #c) p", want: "[1 0 1] 0 .b p|3|p #c p|3|p"},
		{sel: "div > :is(.a .b)", want: "[0 2 1] 0 div > :not(:not(.a .b))|0|"},
		{sel: "p.c:where(p, a)", want: "[0 1 1] 0 p.c|2|c"},
		{sel: "a::before", want: "[0 0 2] 1 a|3|a"},
		{sel: "a:first-child", want: "[0 1 1] 0 a:first-child|3|a"},
		{sel: "a:first-child", m: juice, want: "[0 0 2] 0 a:first-child|3|a"},
		{sel: "a:not(.x)", want: "[0 1 1] 0 a:not(.x)|3|a"},
		{sel: "[width=600]", m: Mode{Lenient: true}, want: `[0 1 0] 0 [width="600"]|0|`},
		{sel: "[width=600]", want: "[0 0 0] 0"},
		{sel: ":has(> b)", want: "[0 0 1] 0 :haschild(b)|0|"},
		{sel: "P#i.c", want: "[1 1 1] 0 P#i.c|1|i"},
		{sel: ".c\\+P", want: `[0 1 0] 0 .c\+P|0|`},
		{sel: ".a.a .a", m: Mode{Juice9: true}, want: "[0 2 0] 0 .a.a .a|2|a"},
		{sel: "a:hover", ignored: true},
		{sel: "a:is(b", want: "[0 0 0] 0"},
		{sel: "a[x", want: "[0 0 0] 0"},
	} {
		got := Compile([]byte(c.sel), c.m)
		if got.Ignored != c.ignored {
			t.Errorf("%q: ignored %v, want %v", c.sel, got.Ignored, c.ignored)
			continue
		}
		if c.ignored {
			continue
		}
		s := fmt.Sprintf("%v %d", got.Spec, got.Pseudo)
		for _, v := range got.Variants {
			s += fmt.Sprintf(" %s|%d|%s", v.Text, v.Key, v.KeyName)
		}
		if s != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.sel, s, c.want)
		}
	}
}

func TestSplit(t *testing.T) {
	got := Split([]byte(` a, b[x=","] , :is(c, d) ,, e;`))
	want := []string{"a", `b[x=","]`, ":is(c, d)", "e"}
	if !slices.EqualFunc(got, want, func(g []byte, w string) bool { return string(g) == w }) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestCompileLinear compiles selectors nested n and 8n deep. Each level is
// read once, so 8x the depth takes about 8x the time.
func TestCompileLinear(t *testing.T) {
	for _, open := range []string{":is(", ":not(:is(", ":has(", ":nth-child(2n of "} {
		sel := func(n int) []byte {
			return []byte(strings.Repeat(open, n) + "p" + strings.Repeat(")", n*strings.Count(open, "(")))
		}
		small, large := fastest(sel(100)), fastest(sel(800))
		if ratio := float64(large) / float64(small); ratio > 24 {
			t.Errorf("%s: 8x the depth took %.0fx the time (%v vs %v)", open, ratio, large, small)
		}
	}
}

func fastest(sel []byte) time.Duration {
	best := time.Duration(1<<63 - 1)
	for range 5 {
		start := time.Now()
		Compile(sel, Mode{})
		best = min(best, time.Since(start))
	}
	return best
}

// SplitList splits a list where it is not nested, and refuses a selector
// whose argument lists, or stray parentheses, nest deeper than its limit.
func TestSplitList(t *testing.T) {
	nest := func(open string, n int) string {
		return strings.Repeat(open, n) + "a" + strings.Repeat(")", n)
	}
	for _, c := range []struct {
		sel  string
		want []string // nil when refused
	}{
		{"a, b:not(c, d), [x=\",\"]", []string{"a", " b:not(c, d)", " [x=\",\"]"}},
		{nest(":not(", 7), []string{nest(":not(", 7)}},
		{nest(":not(", 8), nil},
		{nest("(", 7), []string{nest("(", 7)}},
		{nest("(", 8), nil},
		{":not(" + nest("(", 6) + ")", []string{":not(" + nest("(", 6) + ")"}},
		{":not(" + nest("(", 7) + ")", nil},
		{"b, " + nest(":is(", 8), nil},
	} {
		got, ok := SplitList([]byte(c.sel), 7)
		if c.want == nil {
			if ok {
				t.Errorf("%q: split into %q, want it refused", c.sel, got)
			}
			continue
		}
		if !ok || !slices.EqualFunc(got, c.want, func(g []byte, w string) bool { return string(g) == w }) {
			t.Errorf("%q: got %q (%v), want %q", c.sel, got, ok, c.want)
		}
	}
}

// A selector of :is() lists expands into at most 32 selectors per list it
// holds: once a pass holds 32, the lists left stay whole. Without the cap,
// n lists of two alternatives give 2^n selectors (4,096 here). The lists
// are joined by descendant combinators: a compound of them,
// :is(a, b):is(a, b), gives two selectors only.
func TestExpansionIsBounded(t *testing.T) {
	const lists = 12
	sel := strings.Repeat(":is(a, b) ", lists)
	c := Compile([]byte(sel), Mode{})
	if n := len(c.Variants); n == 0 || n > 32*lists {
		t.Errorf("%d variants", n)
	}
}
