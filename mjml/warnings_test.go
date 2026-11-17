package mjml

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestValidationWarnings checks the warnings of testdata/validation-warnings:
// the messages and lines are those MJML 5.4.1 reports with soft validation
// (mjml2html(..., {validationLevel: "soft"}).errors), in document order, plus
// safe-mjml's own warning for mj-html-attributes, which MJML supports and safe-mjml
// does not. None of them makes Render fail.
func TestValidationWarnings(t *testing.T) {
	content, err := os.ReadFile("testdata/validation-warnings.mjml")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"line 3 (mj-html-attributes) [unsupported-tag] Element mj-html-attributes is not supported by safe-mjml and is ignored",
		"line 9 (mj-text) [misplaced-tag] mj-text cannot be used inside mj-section, only inside: mj-attributes, mj-column, mj-hero",
		"line 11 (mj-foo) [unknown-tag] Element mj-foo doesn't exist or is not registered",
		"line 12 (mj-text) [invalid-attribute] Attribute foo is illegal",
		"line 13 (mj-column) [misplaced-tag] mj-column cannot be used inside mj-column, only inside: mj-attributes, mj-group, mj-section",
		"line 16 (mj-spacer) [misplaced-tag] mj-spacer cannot be used inside mj-body, only inside: mj-attributes, mj-column, mj-hero",
	}
	format := func(warnings []Warning) []string {
		out := make([]string, len(warnings))
		for i, w := range warnings {
			out[i] = fmt.Sprintf("line %d (%s) [%s] %s", w.Line, w.TagName, w.Kind, w.Message)
		}
		return out
	}

	var handled []Warning
	html, err := Render(string(content), withWarningHandler(func(w Warning) { handled = append(handled, w) }))
	if err != nil {
		t.Fatalf("Render must not fail on validation problems, got: %v", err)
	}
	if html == "" {
		t.Fatal("Render returned no HTML")
	}
	if got := format(handled); !reflect.DeepEqual(got, want) {
		t.Errorf("warnings passed to the handler:\n got: %q\nwant: %q", got, want)
	}

	result, err := Compile(string(content))
	if err != nil {
		t.Fatalf("RenderWithAST: %v", err)
	}
	if got := format(result.Warnings); !reflect.DeepEqual(got, want) {
		t.Errorf("RenderResult.Warnings:\n got: %q\nwant: %q", got, want)
	}
}

// TestNoWarningsForValidDocuments checks that a valid document reports nothing.
func TestNoWarningsForValidDocuments(t *testing.T) {
	result, err := Compile(`<mjml><mj-head><mj-attributes><mj-all font-family="Arial" /><mj-class name="x" color="red" /></mj-attributes></mj-head>` +
		`<mj-body><mj-wrapper><mj-section><mj-column><mj-text mj-class="x" css-class="y">Hi</mj-text></mj-column></mj-section></mj-wrapper></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %+v", result.Warnings)
	}
}

// TestNestedSectionContentVisible guards against a nested section leaving an
// Outlook conditional open, which hid everything after it outside Outlook.
func TestNestedSectionContentVisible(t *testing.T) {
	content, err := os.ReadFile("testdata/mj-section-nested-in-column.mjml")
	if err != nil {
		t.Fatal(err)
	}
	html, err := Render(string(content))
	if err != nil {
		t.Fatal(err)
	}
	visible := outlookOnly.ReplaceAllString(html, "")
	for _, text := range []string{"Nested section", "After the nested section", "Next section"} {
		if !strings.Contains(visible, text) {
			t.Errorf("%q is only visible in Outlook", text)
		}
	}
}

// outlookOnly matches the content of Outlook-only conditional comments.
var outlookOnly = regexp.MustCompile(`(?s)<!--\[if mso \| IE\]>.*?<!\[endif\]-->`)

// TestRootWarningsInDocumentOrder checks the warnings of tags directly under
// mjml, in document order across the root and the body. MJML reports the
// unknown tags the same way and drops the known misplaced ones silently;
// safe-mjml also says why they are dropped.
func TestRootWarningsInDocumentOrder(t *testing.T) {
	doc := "<mjml>\n<mj-body>\n<mj-section><mj-column><mj-foo/></mj-column></mj-section>\n</mj-body>\n" +
		"<mj-title>x</mj-title>\n<mj-bar/>\n<mj-section><mj-column><mj-text>ROOT</mj-text></mj-column></mj-section>\n</mjml>\n"
	want := []string{
		"line 3 (mj-foo) [unknown-tag] Element mj-foo doesn't exist or is not registered",
		"line 5 (mj-title) [misplaced-tag] mj-title cannot be used inside mjml, only inside: mj-attributes, mj-head",
		"line 6 (mj-bar) [unknown-tag] Element mj-bar doesn't exist or is not registered",
		"line 7 (mj-section) [misplaced-tag] mj-section cannot be used inside mjml, only inside: mj-attributes, mj-body, mj-wrapper",
	}
	var got []string
	html, err := Render(doc, withWarningHandler(func(w Warning) {
		got = append(got, fmt.Sprintf("line %d (%s) [%s] %s", w.Line, w.TagName, w.Kind, w.Message))
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("warnings:\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(html, "ROOT") {
		t.Error("a section directly under mjml must not be rendered, as in MJML")
	}
}

// TestWarningsWithoutBody checks that the error of a document without
// mj-body comes with the warnings that explain it.
func TestWarningsWithoutBody(t *testing.T) {
	_, err := Compile("<mjml><mj-bdy><mj-section/></mj-bdy></mjml>")
	var mjmlErr *Error
	if !errors.As(err, &mjmlErr) {
		t.Fatalf("expected an *Error, got %v", err)
	}
	if len(mjmlErr.Warnings) != 1 || mjmlErr.Warnings[0].TagName != "mj-bdy" || mjmlErr.Warnings[0].Kind != WarningUnknownTag {
		t.Errorf("got warnings %+v, want the unknown mj-bdy", mjmlErr.Warnings)
	}
}

// TestWarningsOrderOnOneLine checks document order for minified MJML, where
// every element is on line 1.
func TestWarningsOrderOnOneLine(t *testing.T) {
	doc := `<mjml><mj-body><mj-section><mj-column><mj-foo/><mj-text bad="1">x</mj-text></mj-column></mj-section></mj-body><mj-title>x</mj-title></mjml>`
	var got []string
	if _, err := Render(doc, withWarningHandler(func(w Warning) { got = append(got, w.TagName) })); err != nil {
		t.Fatal(err)
	}
	if want := []string{"mj-foo", "mj-text", "mj-title"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestWarningsNeverNil checks that a clean document has an empty, non-nil
// list of warnings, which encodes to [] rather than null.
func TestWarningsNeverNil(t *testing.T) {
	result, err := Compile(`<mjml><mj-body><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>`)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result.Warnings)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "[]" {
		t.Errorf("Warnings encodes to %s, want []", encoded)
	}
}

// TestAttributeDefinitionWarnings checks testdata/validation-attribute-definitions
// against MJML 5.4.1's soft validation: the attributes defined for a component
// in mj-attributes are validated (mj-all and mj-class are not), an unknown tag
// there is reported, and a start tag written over several lines is reported at
// the line of its '<'.
func TestAttributeDefinitionWarnings(t *testing.T) {
	content, err := os.ReadFile("testdata/validation-attribute-definitions.mjml")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Compile(string(content))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range result.Warnings {
		got = append(got, fmt.Sprintf("line %d (%s) %s", w.Line, w.TagName, w.Message))
	}
	want := []string{
		"line 4 (mj-text) Attribute foo is illegal",
		"line 7 (mj-qux) Element mj-qux doesn't exist or is not registered",
		"line 12 (mj-text) Attribute foo is illegal",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("warnings:\n got: %q\nwant: %q", got, want)
	}
}

// TestWarningLineAfterMultilineCloseTag checks that a close tag written over
// two lines (</mj-text⏎>, as Prettier writes MJML) keeps the lines of the
// warnings after it, as MJML 5.4.1 reports them (line 8 here).
func TestWarningLineAfterMultilineCloseTag(t *testing.T) {
	doc := "<mjml>\n  <mj-body>\n    <mj-section>\n      <mj-column>\n        <mj-text>a</mj-text\n        >\n        <mj-text>b</mj-text>\n        <mj-text foo=\"x\">c</mj-text>\n      </mj-column>\n    </mj-section>\n  </mj-body>\n</mjml>\n"
	result, err := Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Line != 8 {
		t.Fatalf("want one warning on line 8, got %+v", result.Warnings)
	}
}

// TestIncludeWarnings checks that every mj-include gets safe-mjml's
// warning, wherever the parser removed it from: under mjml, in an
// mj-class, in an unknown tag, in a second mj-body, and nested in another.
func TestIncludeWarnings(t *testing.T) {
	template := `<mjml>
<mj-include path="root" />
<mj-head><mj-attributes><mj-class name="c" color="green"><mj-include path="class" /></mj-class></mj-attributes></mj-head>
<mj-body>
<mj-section><mj-column><mj-unknown><mj-include path="unknown" /></mj-unknown></mj-column></mj-section>
<mj-include path="outer"><mj-include path="inner" /></mj-include>
</mj-body>
<mj-body><mj-include path="second-body" /></mj-body>
</mjml>`
	result, err := Compile(template)
	if err != nil {
		t.Fatal(err)
	}
	var lines []int
	for _, w := range result.Warnings {
		if w.TagName == "mj-include" {
			lines = append(lines, w.Line)
		}
	}
	if want := []int{2, 3, 5, 6, 6, 8}; !reflect.DeepEqual(lines, want) {
		t.Errorf("mj-include warnings on lines %v, want %v (all: %v)", lines, want, result.Warnings)
	}
}
