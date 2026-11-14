package command

import (
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/mjml"
)

func TestRenderWithWarningsPrintsWarnings(t *testing.T) {
	var out strings.Builder
	doc := "<mjml>\n<mj-body>\n<mj-section><mj-column><mj-text foo=\"bar\">x</mj-text></mj-column></mj-section>\n</mj-body>\n</mjml>"
	html, warnings, err := renderWithWarnings(doc, "in.mjml", &out)
	if err != nil {
		t.Fatal(err)
	}
	if warnings != 1 {
		t.Errorf("got %d warnings, want 1", warnings)
	}
	if want := "in.mjml:3: warning: (mj-text) Attribute foo is illegal\n"; out.String() != want {
		t.Errorf("stderr: got %q, want %q", out.String(), want)
	}
	if !strings.Contains(html, ">x<") {
		t.Error("the document must still be rendered")
	}
}

func TestRenderWithWarningsQuietForValidDocument(t *testing.T) {
	var out strings.Builder
	_, warnings, err := renderWithWarnings("<mjml><mj-body><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>", "in.mjml", &out)
	if err != nil || warnings != 0 || out.Len() != 0 {
		t.Errorf("got err=%v warnings=%d output=%q, want nothing", err, warnings, out.String())
	}
}

func TestRenderWithWarningsPrintsWarningsBeforeAFailure(t *testing.T) {
	var out strings.Builder
	doc := "<mjml>\n<mj-head>\n<mj-attributes><mj-text foo=\"bar\" /></mj-attributes>\n</mj-head>\n</mjml>"
	_, warnings, err := renderWithWarnings(doc, "in.mjml", &out)
	if err == nil {
		t.Fatal("a document without mj-body must fail")
	}
	if warnings != 1 {
		t.Errorf("got %d warnings, want 1", warnings)
	}
	if want := "in.mjml:3: warning: (mj-text) Attribute foo is illegal\n"; out.String() != want {
		t.Errorf("stderr: got %q, want %q", out.String(), want)
	}
}

// Invalid attribute values count as warnings, so that --strict fails on
// them, as MJML's strict validation does.
func TestRenderWithWarningsCountsInvalidValues(t *testing.T) {
	var out strings.Builder
	doc := `<mjml><mj-body><mj-section><mj-column><mj-text align="bogus" font-size="banana" color="nonsense">x</mj-text></mj-column></mj-section></mj-body></mjml>`
	_, warnings, err := renderWithWarnings(doc, "in.mjml", &out)
	if err != nil {
		t.Fatal(err)
	}
	if warnings != 3 {
		t.Errorf("got %d warnings, want 3:\n%s", warnings, out.String())
	}
}

// TestPrintRenderErrorShowsTheStackOfAPanic checks that the CLI prints the
// stack of an internal error, for a bug report, and only for one.
func TestPrintRenderErrorShowsTheStackOfAPanic(t *testing.T) {
	carousel := `<mjml><mj-body><mj-section><mj-column><mj-carousel><mj-carousel-image src="https://e.com/a.png" /></mj-carousel></mj-column></mj-section></mj-body></mjml>`
	_, err := mjml.Render(carousel, mjml.WithIDGenerator(func() string { panic("boom") }))
	var out strings.Builder
	printRenderError(&out, err)
	if !strings.Contains(out.String(), "internal error") || !strings.Contains(out.String(), "goroutine") {
		t.Errorf("no stack for an internal error:\n%s", out.String())
	}
	_, err = mjml.Render("<mjml></mjml>")
	out.Reset()
	printRenderError(&out, err)
	if strings.Contains(out.String(), "goroutine") || !strings.Contains(out.String(), "mj-body") {
		t.Errorf("an ordinary error is printed with a stack, or without its message:\n%s", out.String())
	}
}
