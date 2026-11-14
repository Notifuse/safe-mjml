package juicer

import "testing"

func TestMaybeColor(t *testing.T) {
	for v, want := range map[string]bool{
		"#fff":                          true,
		"#F8F6F3":                       true,
		"#ffff":                         true,
		"#ff":                           false,
		"#ggg":                          false,
		"red":                           true,
		"rgb(250, 250, 250)":            true,
		"rgba(0,0,0,.5)":                true,
		"rgb(250 250 250 / 15%)":        true,
		"rgb(calc(100 + 1) 2 3)":        true,
		"HSL(1 2% 3%)":                  true,
		"rgb(111 222 / 1)":              false,
		"rgb(1, 2)":                     false,
		"rgb(1 2 3 4)":                  false,
		"rgb(1 2 3":                     false,
		"oklch(70% 0.1 200)":            true,
		"color-mix(in srgb, red, blue)": true,
	} {
		if got := maybeColor(v); got != want {
			t.Errorf("maybeColor(%q) = %v, want %v", v, got, want)
		}
	}
}
