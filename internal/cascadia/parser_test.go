package cascadia

import (
	"strings"
	"testing"
)

var identifierTests = map[string]string{
	"x":             "x",
	"96":            "",
	"-x":            "-x",
	`r\e9 sumé`:     "résumé",
	`r\0000e9 sumé`: "résumé",
	`r\0000e9sumé`:  "résumé",
	`a\"b`:          `a"b`,
}

func TestParseIdentifier(t *testing.T) {
	for source, want := range identifierTests {
		p := &parser{s: source}
		got, err := p.parseIdentifier()
		if err != nil {
			if want == "" {
				// It was supposed to be an error.
				continue
			}
			t.Errorf("parsing %q: got error (%s), want %q", source, err, want)
			continue
		}

		if want == "" {
			if err == nil {
				t.Errorf("parsing %q: got %q, want error", source, got)
			}
			continue
		}

		if p.i < len(source) {
			t.Errorf("parsing %q: %d bytes left over", source, len(source)-p.i)
			continue
		}

		if got != want {
			t.Errorf("parsing %q: got %q, want %q", source, got, want)
		}
	}
}

var stringTests = map[string]string{
	`"x"`:             "x",
	`'x'`:             "x",
	`'x`:              "",
	"'x\\\r\nx'":      "xx",
	`"r\e9 sumé"`:     "résumé",
	`"r\0000e9 sumé"`: "résumé",
	`"r\0000e9sumé"`:  "résumé",
	`"a\"b"`:          `a"b`,
}

func TestParseString(t *testing.T) {
	for source, want := range stringTests {
		p := &parser{s: source}
		got, err := p.parseString()
		if err != nil {
			if want == "" {
				// It was supposed to be an error.
				continue
			}
			t.Errorf("parsing %q: got error (%s), want %q", source, err, want)
			continue
		}

		if want == "" {
			if err == nil {
				t.Errorf("parsing %q: got %q, want error", source, got)
			}
			continue
		}

		if p.i < len(source) {
			t.Errorf("parsing %q: %d bytes left over", source, len(source)-p.i)
			continue
		}

		if got != want {
			t.Errorf("parsing %q: got %q, want %q", source, got, want)
		}
	}
}

// A selector nested more than 1,000 deep is refused rather than parsed:
// each level recurses.
func TestParseDepthIsBounded(t *testing.T) {
	nest := func(n int) string {
		return strings.Repeat(":not(", n) + "a" + strings.Repeat(")", n)
	}
	if _, err := Compile(nest(997)); err != nil {
		t.Errorf("997 deep: %v", err)
	}
	if _, err := Compile(nest(1003)); err == nil {
		t.Error("1,003 deep: parsed")
	}
}
