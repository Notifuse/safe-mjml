package components

import "testing"

func TestHasPositiveCellSpacing(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"0px":   false,
		"4":     true,
		"4px":   true,
		" 2 ":   true,
		"0.5":   true,
		"1.2.3": true,
		"abc":   false,
		".":     false,
	}
	for value, want := range cases {
		if got := hasPositiveCellSpacing(value); got != want {
			t.Errorf("hasPositiveCellSpacing(%q) = %v, want %v", value, got, want)
		}
	}
}
