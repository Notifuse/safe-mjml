package components

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// referenceMatchers are mjml-core's types as regular expressions only, the
// way types/*.js test a value: the oracle of TestAttributeTypesMatchMJMLCore.
func referenceMatchers(spec string) (matchers []*regexp.Regexp, exact []string, any bool) {
	lower := strings.ToLower(spec)
	switch {
	case strings.HasPrefix(lower, "boolean"):
		return nil, []string{"true", "false"}, false
	case strings.HasPrefix(lower, "enum"):
		for _, v := range strings.Split(parenthesized(spec), ",") {
			matchers = append(matchers, regexp.MustCompile("^"+regexp.QuoteMeta(v)+"$"))
		}
		return matchers, nil, false
	case strings.HasPrefix(lower, "color"):
		var names []string
		for n := range colorNames {
			names = append(names, n)
		}
		e := rgbExpressions()
		return []*regexp.Regexp{e[0], e[1],
			regexp.MustCompile(`(?i)^#([0-9a-f]{3}){1,2}$`),
			regexp.MustCompile(`^(` + strings.Join(names, "|") + `)$`)}, nil, false
	case isUnitSpec(lower):
		return []*regexp.Regexp{newUnitType(spec).unit.expression()}, nil, false
	case strings.HasPrefix(lower, "integer"):
		return []*regexp.Regexp{regexp.MustCompile(`\d+`)}, nil, false
	}
	return nil, nil, true
}

// TestAttributeTypesMatchMJMLCore checks that the checkers, with their fast
// paths, give mjml-core's expressions' result on every spec of
// allowed-css-attributes.json, for usual, odd and random values.
func TestAttributeTypesMatchMJMLCore(t *testing.T) {
	fragments := []string{"", " ", "  ", "0", "00", "auto", "10", "10px", "1.5em", "-2em", "-", ".", ",", "%", "10%", "px", "em",
		"red", "Red", "transparent", "#abc", "#ABCDEF", "#abcd", "#ab", "rgb(1,2,3)", "rgba(1, 2, 3, 0.5)", "RGB(1,2,3)", "rgb(", "x",
		"true", "false", "left", "center", "full-width", "none", "4", " ", "\t", "1,5px", "1px2px"}
	values := append([]string{}, fragments...)
	r := rand.New(rand.NewSource(1))
	for range 20000 {
		var b strings.Builder
		for range 1 + r.Intn(5) {
			b.WriteString(fragments[r.Intn(len(fragments))])
		}
		values = append(values, b.String())
	}
	specs := map[string]bool{}
	for _, attrs := range allowedAttributes() {
		for _, a := range attrs {
			specs[a.spec] = true
		}
	}
	for spec := range specs {
		checker := newAttributeType(spec)
		matchers, exact, anyValue := referenceMatchers(spec)
		for _, v := range values {
			want := anyValue
			for _, e := range exact {
				want = want || v == e
			}
			for _, m := range matchers {
				want = want || m.MatchString(v)
			}
			if got := checker.valid(v); got != want {
				t.Errorf("%s %q: valid=%v, mjml-core's expressions say %v", spec, v, got, want)
			}
		}
	}
}
