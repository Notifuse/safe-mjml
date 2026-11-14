package fonts

import (
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	lato   = "https://fonts.googleapis.com/css?family=Lato:300,400,500,700"
	roboto = "https://fonts.googleapis.com/css?family=Roboto:300,400,500,700"
	ubuntu = "https://fonts.googleapis.com/css?family=Ubuntu:300,400,500,700"
	droid  = "https://fonts.googleapis.com/css?family=Droid+Sans:300,400,500,700"
)

// TestToImportAsMJML checks ToImport against the patterns of mjml-core's
// buildFontsTags.
func TestToImportAsMJML(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		inline       []string
		declarations []Declaration
		want         []string
	}{
		{"none", `<div style="color:red">Ubuntu</div>`, nil, nil, nil},
		{"in a style attribute", `<div style="font-family:Ubuntu, Helvetica">x</div>`, nil, nil, []string{ubuntu}},
		{"any case", `<p STYLE="Font-Family:'LATO', serif">x</p>`, nil, nil, []string{lato}},
		{"registry order", `<p style="font-family:Ubuntu"></p><p style="font-family:Droid Sans"></p><p style="font-family:Lato"></p>`, nil, nil, []string{droid, lato, ubuntu}},
		{"after font-family only", `<p style="color:Ubuntu;font-family:Arial"></p>`, nil, nil, nil},
		{"in the same quotes", `<p style="font-family:Arial">Ubuntu</p>`, nil, nil, nil},
		{"empty", `<p style="font-family:;"></p>`, nil, nil, nil},
		{"no quote before", `font-family:Ubuntu"`, nil, nil, nil},
		{"no closing quote", `<p style="font-family:Ubuntu`, nil, nil, nil},
		{"inline style", ``, []string{`.a { font-family: Roboto; }`}, nil, []string{roboto}},
		{"inline style stops at ;", ``, []string{`.a { font-family:Arial; color:Roboto }`}, nil, nil},
		{"declared and used", `<p style="font-family:Brand"></p>`, nil, []Declaration{{"Brand", "https://x/brand.css"}}, []string{"https://x/brand.css"}},
		{"declared, unused", `<p style="font-family:Arial"></p>`, nil, []Declaration{{"Brand", "https://x/brand.css"}}, nil},
		{"declaration replaces in place", `<p style="font-family:Lato, Ubuntu"></p>`, nil, []Declaration{{"Lato", "https://x/lato.css"}}, []string{"https://x/lato.css", ubuntu}},
		{"case-sensitive replacement", `<p style="font-family:Lato"></p>`, nil, []Declaration{{"lato", "https://x/lato.css"}}, []string{lato, "https://x/lato.css"}},
		{"a name in another case, not ASCII", `<p style="font-family:'ÉCOLE', serif">x</p>`, nil, []Declaration{{"École", "https://x/ecole.css"}}, []string{"https://x/ecole.css"}},
		{"FONT-FAMILY in capitals", `<p style="FONT-FAMILY:ROBOTO">x</p>`, nil, nil, []string{roboto}},
	}
	for _, tt := range tests {
		if got := ToImport(tt.body, tt.inline, tt.declarations); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: ToImport = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestToImportMatchesMJMLPatterns compares ToImport with mjml-core's
// regular expressions, compiled as written, on 100,000 generated bodies and
// inline styles made of the pieces that matter (quotes, stops, font-family:
// in any case, font names, with and without mj-font declarations).
func TestToImportMatchesMJMLPatterns(t *testing.T) {
	pieces := []string{`"`, `'`, `;`, `}`, "\n", "font-family:", "FONT-FAMILY:", "Font-Family:", "not-font-family:", "font-family :", "font-familY:", "<", ">", "Ubuntu", "uBUNTU", "Roboto", "ROBOTO", "Lato", "Droid Sans", "Open Sans", "Arial", "Brand", "brand x", "#a{}", "color:red;", "none", "font-family:font-family:", "École", "ÉCOLE", "école", "Ü"}
	declarations := []Declaration{{"Brand", "https://x/brand.css"}, {"Lato", "https://x/lato.css"}, {"brand x", "https://x/bx.css"}, {"École", "https://x/ecole.css"}}
	r := rand.New(rand.NewSource(815))
	for n := range 100000 {
		var body, css strings.Builder
		for range r.Intn(35) {
			body.WriteString(pieces[r.Intn(len(pieces))])
		}
		for range r.Intn(25) {
			css.WriteString(pieces[r.Intn(len(pieces))])
		}
		var decls []Declaration
		if n%2 == 1 {
			decls = declarations
		}
		var want []string
		for _, f := range patterns(decls) {
			if f.body.MatchString(body.String()) || f.inline.MatchString(css.String()) {
				want = append(want, f.href)
			}
		}
		if got := ToImport(body.String(), []string{css.String()}, decls); !reflect.DeepEqual(got, want) {
			t.Fatalf("sample %d: body=%q inline=%q: got %v, want %v", n, body.String(), css.String(), got, want)
		}
	}
}

type fontPattern struct {
	href         string
	body, inline *regexp.Regexp
}

var patternCache = map[int][]fontPattern{}

// patterns returns mjml-core's two regular expressions for each font of
// the registry with decls, compiled once per number of declarations.
func patterns(decls []Declaration) []fontPattern {
	if p, ok := patternCache[len(decls)]; ok {
		return p
	}
	var p []fontPattern
	for _, f := range buildRegistry(decls) {
		name := regexp.QuoteMeta(f.name)
		p = append(p, fontPattern{f.href,
			regexp.MustCompile(`(?i)"[^"]*font-family:[^"]*` + name + `[^"]*"`),
			regexp.MustCompile(`(?i)font-family:[^;}]*` + name)})
	}
	patternCache[len(decls)] = p
	return p
}

// TestToImportIsLinear checks the inputs that cost quadratic time before:
// repeated font-family: in one attribute or one inline rule, a long font
// name that almost matches, and many declared fonts against many styles.
func TestToImportIsLinear(t *testing.T) {
	const n = 32000
	cases := map[string]func() (string, []string, []Declaration){
		"repeated font-family: in an attribute": func() (string, []string, []Declaration) {
			return `<span title="` + strings.Repeat("font-family:", n) + `">x</span>`, nil, nil
		},
		"repeated font-family: in an inline rule": func() (string, []string, []Declaration) {
			return "", []string{".a{" + strings.Repeat("font-family:", n) + "}"}, nil
		},
		"a long name that almost matches": func() (string, []string, []Declaration) {
			return `<p style="font-family:` + strings.Repeat("a", 2*n) + `">x</p>`, nil, []Declaration{{strings.Repeat("a", n) + "b", "https://x/a.css"}}
		},
		"many fonts and many styles": func() (string, []string, []Declaration) {
			var decls []Declaration
			for i := range 8000 {
				decls = append(decls, Declaration{fmt.Sprintf("Font%d", i), fmt.Sprintf("https://x/%d.css", i)})
			}
			return strings.Repeat(`<p style="font-family:Arial">x</p>`, 20000), nil, decls
		},
	}
	for name, input := range cases {
		body, inline, decls := input()
		start := time.Now()
		ToImport(body, inline, decls)
		if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
			t.Errorf("%s: %v", name, elapsed)
		}
	}
}

// ToImport reads the body where it is: it allocated a lowered copy of it on
// every render before, the larger part of the render's time for a body with
// non-ASCII text (strings.ToLower then maps rune by rune). Only the texts
// after font-family: may be copied, when they hold capitals.
func TestToImportDoesNotCopyTheBody(t *testing.T) {
	cell := `<td style="font-family:Ubuntu, Helvetica, Arial, sans-serif;font-size:13px;color:#000000;"><div>Écrire à l'équipe — Lorem ipsum dolor sit amet</div></td>`
	body := strings.Repeat(cell, 8000) // about 1 MB
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if got := ToImport(body, nil, nil); !reflect.DeepEqual(got, []string{ubuntu}) {
		t.Fatalf("ToImport = %v", got)
	}
	runtime.ReadMemStats(&after)
	if bytes := after.TotalAlloc - before.TotalAlloc; bytes > uint64(len(body)/4) {
		t.Errorf("ToImport allocated %d bytes for a body of %d bytes", bytes, len(body))
	}
}
