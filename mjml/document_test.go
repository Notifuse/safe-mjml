package mjml_test

import (
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/mjml"
)

// A Document compiles and renders as Compile and Render do with its
// template.
func TestDocumentCompileAndRender(t *testing.T) {
	const template = `<mjml><mj-body><mj-section><mj-column><mj-text colour="red">Hello</mj-text></mj-column></mj-section></mj-body></mjml>`
	doc, err := mjml.Parse(template)
	if err != nil {
		t.Fatal(err)
	}
	wantHTML, err := mjml.Render(template)
	if err != nil {
		t.Fatal(err)
	}
	html, err := doc.Render()
	if err != nil || html != wantHTML {
		t.Fatalf("Document.Render differs from Render: %v", err)
	}
	want, err := mjml.Compile(template)
	if err != nil {
		t.Fatal(err)
	}
	res, err := doc.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if res.HTML != want.HTML || len(res.Warnings) != 1 || res.Warnings[0] != want.Warnings[0] {
		t.Fatalf("Document.Compile = %+v, want %+v", res, want)
	}
}

func TestResultJSON(t *testing.T) {
	res := mjml.Result{HTML: "x", Warnings: []mjml.Warning{{Kind: mjml.WarningUnknownTag, TagName: "mj-x", Line: 2, Message: "m"}}}
	out, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"html":"x","warnings":[{"kind":"unknown-tag","tagName":"mj-x","line":2,"message":"m"}]}`; string(out) != want {
		t.Errorf("JSON = %s, want %s", out, want)
	}
}

// A nil or zero Document, Cache or Option does not panic.
func TestZeroValues(t *testing.T) {
	for name, doc := range map[string]*mjml.Document{"nil": nil, "zero": {}} {
		if _, err := doc.Render(); !isMJMLError(err) {
			t.Errorf("%s Document: Render returned %v, want an *mjml.Error", name, err)
		}
		if res, err := doc.Compile(); res != nil || !isMJMLError(err) {
			t.Errorf("%s Document: Compile returned (%v, %v), want an *mjml.Error", name, res, err)
		}
	}

	var nilCache *mjml.Cache
	nilCache.Close()
	(&mjml.Cache{}).Close()
	want, err := mjml.Render(helloTemplate)
	if err != nil {
		t.Fatal(err)
	}
	for name, opt := range map[string]mjml.Option{"nil option": nil, "nil cache": mjml.WithCache(nil), "zero cache": mjml.WithCache(&mjml.Cache{})} {
		if got, err := mjml.Render(helloTemplate, opt); err != nil || got != want {
			t.Errorf("Render with a %s differs: %v", name, err)
		}
	}
}

func isMJMLError(err error) bool {
	var mjmlErr *mjml.Error
	return errors.As(err, &mjmlErr)
}

// A Document does not keep its template alive: an 8 MB comment before the
// mjml root is not part of the parsed tree.
func TestDocumentDoesNotKeepItsTemplate(t *testing.T) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	template := "<!--" + strings.Repeat("x", 8<<20) + "-->" + helloTemplate
	doc, err := mjml.Parse(template)
	if err != nil {
		t.Fatal(err)
	}
	template = ""
	runtime.GC()
	runtime.ReadMemStats(&after)
	if grown := int64(after.HeapAlloc) - int64(before.HeapAlloc); grown > 4<<20 {
		t.Errorf("a parsed Document keeps %d MB alive", grown>>20)
	}
	runtime.KeepAlive(doc)
}
