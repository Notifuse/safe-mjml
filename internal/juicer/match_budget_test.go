package juicer

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// budgetDoc has rules whose descendant combinator walks every ancestor of
// every div, on nested divs: about rules × depth² / 2 steps.
func budgetDoc(rules, depth int) string {
	var css strings.Builder
	for i := range rules {
		fmt.Fprintf(&css, ".absent%d div{color:red}", i)
	}
	return "<style>" + css.String() + "</style>" + strings.Repeat("<div>", depth) + "x" + strings.Repeat("</div>", depth)
}

// MaxMatchSteps refuses a document whose selectors cost more steps than it
// allows, changes nothing below it, and gives each Inline call its own
// budget, so that one Inliner can serve many goroutines.
func TestMaxMatchSteps(t *testing.T) {
	doc := budgetDoc(20, 100) // about 20 × 100² / 2 = 100,000 steps
	free, err := Inline(doc)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := Inline(doc, MaxMatchSteps(1_000_000)); err != nil || out != free {
		t.Errorf("under the limit: %v, or the output changes", err)
	}
	if _, err := Inline(doc, MaxMatchSteps(10_000)); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("over the limit: got %v, want ErrLimitExceeded", err)
	}
	if _, err := New(MaxMatchSteps(-1)); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("negative: got %v, want ErrInvalidOption", err)
	}
	// Each call needs about half of the limit: a budget shared by the calls
	// would run out.
	in := mustNew(t, MaxMatchSteps(250_000))
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				if out, err := in.Inline(doc); err != nil || out != free {
					errs <- fmt.Errorf("concurrent call: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// Each declaration an element gets costs declarationSteps: it is held until
// the document is written, and written into it. A rule of 200 declarations
// on 100 elements is 20,000 of them.
func TestMaxMatchStepsCountsDeclarations(t *testing.T) {
	var decls strings.Builder
	for i := range 200 {
		fmt.Fprintf(&decls, "p%d:1;", i)
	}
	doc := "<style>p{" + decls.String() + "}</style>" + strings.Repeat("<p>x</p>", 100)
	free, err := Inline(doc)
	if err != nil {
		t.Fatal(err)
	}
	const held = 200 * 100 * declarationSteps
	if out, err := Inline(doc, MaxMatchSteps(2*held)); err != nil || out != free {
		t.Errorf("under the limit: %v, or the output changes", err)
	}
	if _, err := Inline(doc, MaxMatchSteps(held/2)); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("over the limit: got %v, want ErrLimitExceeded", err)
	}
}

// A custom property that refers to itself twice doubles at each level; juice
// never finishes it. The substitutions of one value are bounded
// (maxVarSubstitutions), with no step limit.
func TestVarSubstitutionsAreBounded(t *testing.T) {
	out, err := Inline(`<p class="a">x</p>`, CompatibilityMode(Juice10), ApplyStyleTags(false),
		ExtraCSS(".a{--c:var(--c) var(--c);color:var(--c)}"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > 64<<10 {
		t.Errorf("%d bytes written for one self-referring property", len(out))
	}
}

// A copy is paid before it is made: "$`", "$'" and "$&" in a variable, and a
// template block written back many times, built gigabytes and only then
// were refused (the reviews of 4ebbb74). Each document here would make
// 100 MB or more; refused, it allocates little.
func TestCopiesArePaidBeforeTheyAreMade(t *testing.T) {
	in := mustNew(t, CompatibilityMode(Juice10), CodeBlocks(DefaultCodeBlocks()), MaxMatchSteps(1_000_000))
	tail := strings.Repeat("y", 50_000)
	for name, doc := range map[string]string{
		"$` before the call":         "<style>p{--a:" + strings.Repeat("$`", 2000) + ";color:" + tail + " var(--a)}</style><p>x</p>",
		"$' after the call":          "<style>p{--a:" + strings.Repeat("$'", 2000) + ";color:var(--a) " + tail + "}</style><p>x</p>",
		"$&$& in a nested fallback":  "<style>p{margin:var(--a,var(--b,$&$&))}</style><p>x</p>",
		"a block copied by a rule":   "<style>p{a:" + strings.Repeat("juice_code_block_0_ ", 100) + "}</style>{{" + tail + "}}" + strings.Repeat("<p></p>", 50),
		"a block copied in the text": "<style>p{color:red}</style><p>{{" + tail + "}}" + strings.Repeat("juice_code_block_0_ ", 2000) + "</p>",
	} {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := in.Inline(doc)
		runtime.ReadMemStats(&after)
		if !errors.Is(err, ErrLimitExceeded) {
			t.Errorf("%s: got %v, want an error wrapping ErrLimitExceeded", name, err)
		}
		if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 20<<20 {
			t.Errorf("%s: refused after allocating %d MB", name, alloc>>20)
		}
	}
}
