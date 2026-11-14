package juicer

import (
	"errors"
	"strings"
	"testing"
)

// TestDeepDocumentRefusedAsParsed checks that a document nested deeper than
// MaxDepth is refused as the parse reaches the limit, before what follows is
// parsed: refusing it with a long tail allocates no more than refusing it
// without. Measured once parsed, the tail's nodes were allocated first, about
// one slab for 256 elements, and checkDepth refused it after.
func TestDeepDocumentRefusedAsParsed(t *testing.T) {
	in := mustNew(t, MaxDepth(100))
	allocs := func(doc string) float64 {
		return testing.AllocsPerRun(3, func() {
			if _, err := in.Inline(doc); !errors.Is(err, ErrLimitExceeded) {
				t.Fatalf("got %v, want an error wrapping ErrLimitExceeded", err)
			}
		})
	}
	deep := strings.Repeat("<b>", 101)
	short, long := allocs(deep), allocs(deep+strings.Repeat("<i></i>", 100_000))
	if long > short+10 {
		t.Errorf("refusing the document with a tail of 100,000 elements took %.0f allocations, %.0f without", long, short)
	}
}
