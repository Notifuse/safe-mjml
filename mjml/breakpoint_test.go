package mjml

import "testing"

func TestLowerBreakpoint(t *testing.T) {
	cases := map[string]string{
		"480px": "479px",
		"600px": "599px",
		"320":   "319px",
		"30em":  "29px",
		"wide":  "wide",
		"":      "",
	}
	for breakpoint, want := range cases {
		if got := lowerBreakpoint(breakpoint); got != want {
			t.Errorf("lowerBreakpoint(%q) = %q, want %q", breakpoint, got, want)
		}
	}
}
