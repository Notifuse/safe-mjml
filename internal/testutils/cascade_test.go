package testutils

import (
	"maps"
	"reflect"
	"testing"
)

func TestEffectiveDeclarations(t *testing.T) {
	tests := []struct {
		style string
		want  map[string]string
	}{
		{"padding:2px;padding-top:40px", map[string]string{"padding": "2px", "padding-top": "40px"}},
		{"padding-top:40px;padding:2px", map[string]string{"padding": "2px"}},
		{"border-left:none;border:1px solid red", map[string]string{"border": "1px solid red"}},
		{"border:1px solid red;border-left:none", map[string]string{"border": "1px solid red", "border-left": "none"}},
		{"border-radius:3px;border:none", map[string]string{"border-radius": "3px", "border": "none"}},
		{"border-top-left-radius:3px;border-radius:5px", map[string]string{"border-radius": "5px"}},
		{"border-top-color:red;border-top:1px solid", map[string]string{"border-top": "1px solid"}},
		{"border-collapse:separate;border:0", map[string]string{"border-collapse": "separate", "border": "0"}},
		{"background-color:red;background:url(x)", map[string]string{"background": "url(x)"}},
		{"line-height:2;font:12px Arial", map[string]string{"font": "12px Arial"}},
		{"color:red;color:blue", map[string]string{"color": "blue"}},
		{"mso-padding-alt:0;padding:1px", map[string]string{"mso-padding-alt": "0", "padding": "1px"}},
	}
	for _, tt := range tests {
		if got := EffectiveDeclarations(tt.style); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("EffectiveDeclarations(%q) = %v, want %v", tt.style, got, tt.want)
		}
	}
}

// TestEffectiveDeclarationsImportanceAndQuotes checks the !important
// precedence and the quoted semicolons, where a style that a browser applies
// differently must not compare equal.
func TestEffectiveDeclarationsImportanceAndQuotes(t *testing.T) {
	differ := [][2]string{
		{"color:red!important;color:blue", "color:green!important;color:blue"},
		{"padding-top:40px!important;padding:2px", "padding-top:3px!important;padding:2px"},
		{`background-image:url("a;b.png");color:red`, `background-image:url("a;c.png");color:red`},
		{`font-family:'A;B',Arial`, `font-family:'A;C',Arial`},
	}
	for _, c := range differ {
		if maps.Equal(EffectiveDeclarations(c[0]), EffectiveDeclarations(c[1])) {
			t.Errorf("%q and %q compare as equal", c[0], c[1])
		}
	}
	same := [][2]string{
		{"color:red!important;color:blue", "color:red!important"},
		{"padding-top:40px!important;padding:2px", "padding:2px;padding-top:40px!important"},
		{"padding-top:40px;padding:2px", "padding:2px"},
	}
	for _, c := range same {
		if !maps.Equal(EffectiveDeclarations(c[0]), EffectiveDeclarations(c[1])) {
			t.Errorf("%q and %q differ: %v, %v", c[0], c[1], EffectiveDeclarations(c[0]), EffectiveDeclarations(c[1]))
		}
	}
	if got := EffectiveDeclarations(`background-image:url("a;b.png");color:red`); got["color"] != "red" || got["background-image"] != `url("a;b.png")` {
		t.Errorf("quoted semicolon split the value: %v", got)
	}
}
