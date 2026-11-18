package htmlparser2

import (
	"bytes"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// TestParseDepthLimit checks that Parse refuses an element nested deeper
// than maxDepth, as the parse reaches it, whatever made it: a start tag, a
// void element or the <p> of an unmatched </p>.
func TestParseDepthLimit(t *testing.T) {
	for _, c := range []struct {
		doc      string
		maxDepth int
		tooDeep  bool
	}{
		{"<b><b><b>x</b></b></b>", 3, false},
		{"<b></b><b></b><b></b>", 1, false},
		{"<b><b><b><b>", 3, true},
		{"<b><b><b><br>", 3, true},
		{"<b><b><b></p>", 3, true},
		{"<b><b><svg><path/>", 3, true},
		{strings.Repeat("<b>", 100_000), 0, false},
	} {
		root, err := Parse([]byte(c.doc), false, c.maxDepth)
		if got := errors.Is(err, ErrTooDeep); got != c.tooDeep || (err == nil) == (root == nil) {
			t.Errorf("Parse(%.40q, maxDepth %d): %v, refused %v, want %v", c.doc, c.maxDepth, err, got, c.tooDeep)
		}
	}
}

// TestParseScales parses stray close tags under deep nesting, close tags
// with a quoted ">", and a start tag with many attributes. Each stray close
// tag walked every open element, each quoted close tag was read to the end
// of the run, and each attribute was looked up among the tag's others, so 8
// times the input took 64 times as long.
func TestParseScales(t *testing.T) {
	for _, shape := range []struct {
		name string
		n    int
		doc  func(n int) string
	}{
		{"stray close tags", 5000, func(n int) string { return strings.Repeat("<div>", n) + strings.Repeat("</x>", n) }},
		// x/net/html reads a quoted ">" in a close tag as a value and goes
		// on to the next unquoted ">", the end of the run: each close tag
		// was read to there and cut back to its first ">".
		{"quoted close tags", 500, func(n int) string { return strings.Repeat(`</b a='>'`, n) + "<p>x</p>" }},
		{"attributes", 5000, func(n int) string {
			var b strings.Builder
			b.WriteString("<p")
			for i := range n {
				b.WriteString(" a")
				b.WriteString(strings.Repeat("x", i%7))
				b.WriteString(string(rune('a' + i%26)))
				b.WriteString(string(rune('a' + i/26%26)))
				b.WriteString(string(rune('a' + i/676%26)))
			}
			b.WriteString(">")
			return b.String()
		}},
	} {
		t.Run(shape.name, func(t *testing.T) {
			small, _ := fastestParse(shape.doc(shape.n), time.Minute)
			// The large input fails past 24 times the small one's time, so
			// a parse is stopped there rather than left to run a quadratic
			// cost out: one ran for minutes.
			limit := max(24*small, 20*time.Millisecond)
			large, ok := fastestParse(shape.doc(8*shape.n), limit)
			if !ok {
				t.Fatalf("8x the input took more than %v, 24x the time (%v), in each of three parses", limit, small)
			}
			if ratio := float64(large) / float64(max(small, time.Microsecond)); ratio > 24 && large > 20*time.Millisecond {
				t.Errorf("8x the input took %.0fx the time (%v vs %v)", ratio, large, small)
			}
		})
	}
}

// backgroundParses counts the parses of fastestParse still running: a test
// that measures the allocations of the process waits for them first.
var backgroundParses sync.WaitGroup

// fastestParse is the quickest of three parses of doc, or false when each of
// the three runs past limit. A parse past the limit goes on in the
// background while the next one starts (the package's other tests are
// short): a parse descheduled on a loaded machine does not fail the test,
// and a cost that grows too fast is reported after three limits.
func fastestParse(doc string, limit time.Duration) (time.Duration, bool) {
	best, ok := limit, false
	for range 3 {
		done := make(chan time.Duration, 1)
		backgroundParses.Add(1)
		go func() {
			defer backgroundParses.Done()
			start := time.Now()
			if _, err := Parse([]byte(doc), false, 0); err != nil {
				panic(err)
			}
			done <- time.Since(start)
		}()
		select {
		case d := <-done:
			best, ok = min(best, d), true
		case <-time.After(limit):
		}
	}
	return best, ok
}

// TestRepeatedAttributes checks that the first of repeated attributes is
// kept, as htmlparser2 keeps it, with fewer attributes than manyAttributes,
// as many and more, in tags one after the other.
func TestRepeatedAttributes(t *testing.T) {
	var doc, want strings.Builder
	for _, n := range []int{3, manyAttributes - 1, manyAttributes, manyAttributes + 1, 3 * manyAttributes} {
		doc.WriteString("<p")
		want.WriteString("<p")
		for i := range n {
			name := "a" + strings.Repeat("x", i)
			doc.WriteString(" " + name + "=1")
			want.WriteString(" " + name + `="1"`)
		}
		for i := range n {
			doc.WriteString(" a" + strings.Repeat("x", i) + "=2")
		}
		doc.WriteString(` a=3 b="x"></p>`)
		want.WriteString(` b="x"></p>`)
	}
	root, err := Parse([]byte(doc.String()), false, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	Render(&out, root)
	if out.String() != want.String() {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want.String())
	}
}

// TestElementAtoms checks that elements get the DataAtom html.Parse gives
// them, which the pseudo-classes of form elements and links read, whatever
// made the element, and that XML mode keeps the case of names.
func TestElementAtoms(t *testing.T) {
	for _, c := range []struct {
		doc  string
		xml  bool
		want []atom.Atom
	}{
		{`<INPUT checked><a href="#">a</a><x-y></x-y></br></p><svg><title/></svg>`, false,
			[]atom.Atom{atom.Input, atom.A, 0, atom.Br, atom.P, atom.Svg, atom.Title}},
		{`<input/><INPUT/><option>o</option>`, true, []atom.Atom{atom.Input, 0, atom.Option}},
	} {
		root, err := Parse([]byte(c.doc), c.xml, 0)
		if err != nil {
			t.Fatal(err)
		}
		var got []atom.Atom
		Traverse(root, func(n *html.Node) bool {
			if n.Type == html.ElementNode {
				got = append(got, n.DataAtom)
			}
			return true
		}, func(*html.Node) {})
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.doc, got, c.want)
		}
	}
}

// A run of close tags with a quoted ">" is read without restarting the
// tokenizer at each tag, which allocated its 4 KB buffer again each time:
// 4,000 tags (36 KB) allocated 17 MB, and the runner's garbage collector
// made the parse slow enough to fail TestParseScales.
func TestQuotedCloseTagsAllocateLittle(t *testing.T) {
	doc := []byte(strings.Repeat(`</b a='>'`, 4000) + "<p>x</p>")
	// TotalAlloc counts the whole process: a parse TestParseScales left
	// running on a loaded runner made it 4,008 KB, where this parse
	// allocates 486 KB.
	backgroundParses.Wait()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, err := Parse(doc, false, 0); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 2<<20 {
		t.Errorf("parsing 4,000 quoted close tags allocated %d KB", alloc>>10)
	}
}
