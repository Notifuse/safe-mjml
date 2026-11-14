package testutils

import (
	"strings"
	"testing"
)

func TestParseStylesheet(t *testing.T) {
	css := `@import url(https://fonts.googleapis.com/css?family=Ubuntu:300,400);
      /* a comment */
      #outlook a { padding:0; }
      img { border:0;height:auto; outline:none; }
      @media only screen and (min-width: 480px) {
        .mj-column-per-50 { width:50% !important; max-width: 50%; }
      }
      .x { background:url("data:image/png;base64,AAA") }`

	rules := ParseStylesheet(css)
	got := make([]string, len(rules))
	for i, r := range rules {
		got[i] = r.String()
	}
	want := []string{
		"@import url(https://fonts.googleapis.com/css?family=Ubuntu:300,400);",
		"#outlook a { padding:0; }",
		"img { border:0; height:auto; outline:none; }",
		"@media only screen and (min-width:480px) { .mj-column-per-50 { width:50%!important; max-width:50%; } }",
		`.x { background:url("data:image/png;base64,AAA"); }`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected rules:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCompareStylesheets(t *testing.T) {
	base := `.a { width:50%; } .b { width:100%; }`
	tests := []struct {
		name   string
		actual string
		equal  bool
	}{
		{"same rules, other whitespace", ".a{width: 50%}\n.b {width:100%;}", true},
		// The old comparison sorted the characters of the whole stylesheet,
		// so these three passed it.
		{"values swapped between rules", `.a { width:100%; } .b { width:50%; }`, false},
		{"rules in another order", `.b { width:100%; } .a { width:50%; }`, false},
		{"missing rule", `.a { width:50%; }`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := CompareStylesheets(ParseStylesheet(base), ParseStylesheet(tt.actual), "")
			if (len(diffs) == 0) != tt.equal {
				t.Fatalf("equal=%v, diffs=%v", len(diffs) == 0, diffs)
			}
		})
	}

	diffs := CompareStylesheets(
		ParseStylesheet(`.a { color:red; width:1px; }`),
		ParseStylesheet(`.a { width:1px; color:red; }`), "")
	if len(diffs) == 0 {
		t.Fatal("declaration order must be compared")
	}
	diffs = CompareStylesheets(
		ParseStylesheet(`@media (min-width: 480px) { .a { color:red; } }`),
		ParseStylesheet(`@media (min-width:480px) { .a { color:red; } }`), "")
	if len(diffs) != 0 {
		t.Fatalf("media feature whitespace must be ignored: %v", diffs)
	}
}

// TestCompareStylesheetsDifferences lists pairs that must compare different,
// in particular rules after a block, which an earlier version of the parser
// dropped.
func TestCompareStylesheetsDifferences(t *testing.T) {
	tests := []struct{ name, expected, actual string }{
		{"rule after @media with a parenthesis right after the keyword",
			`@media (max-width:480px) { .a{color:red} } .b{color:blue}`,
			`@media (max-width:480px) { .a{color:red} } .b{color:green}`},
		{"rule inside @media with a parenthesis right after the keyword",
			`@media (max-width:480px) { .a{color:red} }`,
			`@media (max-width:480px) { .a{color:blue} }`},
		{"rules in another order inside @media",
			`@media only screen { .a { width:50%; } .b { width:100%; } }`,
			`@media only screen { .b { width:100%; } .a { width:50%; } }`},
		{"rule after @supports",
			`@supports (display:grid) { .a{color:red} } .b{color:blue}`,
			`@supports (display:grid) { .a{color:red} } .b{color:green}`},
		{"rule after @keyframes",
			`@keyframes x { 0% {opacity:0} 100% {opacity:1} } .b{color:blue}`,
			`@keyframes x { 0% {opacity:0} 100% {opacity:1} } .b{color:green}`},
		{"rule after a stray closing brace",
			`.a{color:red} } .b{color:blue}`,
			`.a{color:red} } .b{color:green}`},
		{"stray closing brace",
			`.a{color:red} .b{color:blue}`,
			`.a{color:red} } .b{color:blue}`},
		{"descendant combinator after a parenthesis",
			`.a:not(.b) .c { x:1 }`,
			`.a:not(.b).c { x:1 }`},
		{"whitespace inside a string value",
			`.a { content:"a  b" }`,
			`.a { content:"a b" }`},
		{"whitespace inside a string in a selector",
			`[title="x  y"] { x:1 }`,
			`[title="x y"] { x:1 }`},
		{"comment-like text inside a string",
			`.a { background:url("http://h/*.png") } .b { x:1 } /* c */`,
			`.a { background:url("http://h/*.png") } .b { x:2 } /* c */`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diffs := CompareStylesheets(ParseStylesheet(tt.expected), ParseStylesheet(tt.actual), ""); len(diffs) == 0 {
				t.Fatalf("no difference found between\n  %s\n  %s\nparsed as %v", tt.expected, tt.actual, ParseStylesheet(tt.actual))
			}
		})
	}

	rules := ParseStylesheet(`.a { background:url("http://h/*.png") }`)
	if len(rules) != 1 || rules[0].Declarations[0].Value != `url("http://h/*.png")` {
		t.Fatalf("comment-like text inside a string must be kept: %v", rules)
	}
}
