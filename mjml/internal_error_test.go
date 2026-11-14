package mjml

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Notifuse/safe-mjml/internal/astcache"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestPanicIsAnInternalError checks that a panic during a call does not
// reach the caller: every exported function that parses or compiles returns
// it as an *Error whose cause is ErrInternal, with the stack, and the next
// call works.
func TestPanicIsAnInternalError(t *testing.T) {
	carousel := `<mjml><mj-body><mj-section><mj-column><mj-carousel><mj-carousel-image src="https://e.com/a.png" /><mj-carousel-image src="https://e.com/b.png" /></mj-carousel></mj-column></mj-section></mj-body></mjml>`
	panicking := WithIDGenerator(func() string { panic("generator") })
	check := func(name string, err error) {
		t.Helper()
		var mjmlErr *Error
		switch {
		case !errors.Is(err, ErrInternal):
			t.Errorf("%s: got %v, want an error with ErrInternal as its cause", name, err)
		case !errors.As(err, &mjmlErr):
			t.Errorf("%s: %T is not an *Error", name, err)
		case !strings.Contains(mjmlErr.Message, "internal error") || !strings.Contains(mjmlErr.Message, "generator"):
			t.Errorf("%s: message %q", name, mjmlErr.Message)
		case !strings.Contains(mjmlErr.Stack, "goroutine"):
			t.Errorf("%s: no stack", name)
		case mjmlErr.Details == nil:
			t.Errorf("%s: nil Details", name)
		}
	}
	_, err := Render(carousel, panicking)
	check("Render", err)
	_, err = Compile(carousel, panicking)
	check("Compile", err)
	doc, err := Parse(carousel)
	if err != nil {
		t.Fatal(err)
	}
	_, err = doc.Compile(panicking)
	check("Document.Compile", err)
	_, err = doc.Render(panicking)
	check("Document.Render", err)

	saved := parseMJML
	parseMJML = func(string) (*parser.MJMLNode, error) { panic("generator") }
	_, err = Parse(carousel)
	check("Parse", err)
	_, err = Compile(carousel, WithCache(NewCache(time.Minute)))
	check("Compile with a cache", err)
	parseMJML = saved

	// The JSON of the error has no stack, and Error() neither.
	_, err = Render(carousel, panicking)
	data, _ := json.Marshal(err)
	if strings.Contains(string(data), "goroutine") || strings.Contains(err.Error(), "goroutine") {
		t.Errorf("the stack leaks into the JSON or Error(): %s", data)
	}
	if _, err := Render(carousel); err != nil {
		t.Errorf("a render after a panic failed: %v", err)
	}
}

// TestPanicOfAConcurrentParseIsAnInternalError checks that a call that
// waits, through a shared cache, for a parse of the same template that
// panics gets an ErrInternal too, not an ordinary parse error: a review
// found it got "cannot parse the template: astcache: the parse of the
// template panicked".
func TestPanicOfAConcurrentParseIsAnInternalError(t *testing.T) {
	if err := parseError(astcache.ErrParsePanicked); !errors.Is(err, ErrInternal) {
		t.Fatalf("parseError(ErrParsePanicked) = %v, want an ErrInternal", err)
	}

	template := `<mjml><mj-body><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>`
	inParse, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	saved := parseMJML
	parseMJML = func(string) (*parser.MJMLNode, error) {
		once.Do(func() { close(inParse) })
		<-release
		panic("boom")
	}
	defer func() { parseMJML = saved }()
	cache := NewCache(time.Minute)
	defer cache.Close()

	errs := make(chan error, 2)
	go func() { _, err := Compile(template, WithCache(cache)); errs <- err }()
	<-inParse // the first call parses; the second one waits for it
	go func() { _, err := Compile(template, WithCache(cache)); errs <- err }()
	time.Sleep(20 * time.Millisecond)
	close(release)
	for range 2 {
		if err := <-errs; !errors.Is(err, ErrInternal) {
			t.Errorf("got %v, want an ErrInternal", err)
		}
	}
}
