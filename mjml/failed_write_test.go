package mjml

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/html"
)

var errWriteRefused = errors.New("write refused by the test")

// refusingWriter passes the writes of a render to the document buffer and
// refuses the one numbered refuse (counted from 1, empty writes left out),
// and every write after it, as the buffer refuses them past maxOutputSize.
// With record set, it notes the call path of each write instead.
type refusingWriter struct {
	out    io.StringWriter
	writes int
	refuse int
	record func(write int, path string)
}

func (w *refusingWriter) WriteString(text string) (int, error) {
	if text == "" {
		return w.out.WriteString(text)
	}
	w.writes++
	if w.record != nil {
		w.record(w.writes, callPath())
	}
	if w.refuse > 0 && w.writes >= w.refuse {
		return 0, errWriteRefused
	}
	return w.out.WriteString(text)
}

// callPath names the functions that led to a write, up to the document
// render, without the writers between them and the buffer: two writes with
// the same path return their error through the same branches.
func callPath() string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var path strings.Builder
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "renderDocument") || !more {
			break
		}
		if strings.Contains(frame.File, "internal/html/mso.go") {
			continue // the Outlook merge writer
		}
		path.WriteString(frame.File)
		path.WriteByte(':')
		path.WriteString(strconv.Itoa(frame.Line))
		path.WriteByte('\n')
	}
	return path.String()
}

// renderRefusing renders template, refusing its write numbered refuse.
func renderRefusing(t *testing.T, template string, writer *refusingWriter) error {
	t.Helper()
	ast := mustParse(t, template)
	tree, err := newComponentTree(ast)
	if err != nil {
		return err
	}
	root := tree.(*rootComponent)
	if root.Body == nil {
		return errors.New("no mj-body")
	}
	out := &documentBuffer{}
	writer.out = out
	merge := html.NewOutlookMergeWriter(writer)
	if len(root.inlineStyles()) > 0 {
		merge = html.NewOutlookWriteThrough(writer)
	}
	_, _, err = root.renderDocument(merge, out)
	if err == nil {
		err = merge.Flush()
	}
	return err
}

// TestRefusedWriteStopsTheRender checks that a write the document buffer
// refuses, as it does past maxOutputSize, stops the render: every component
// returns the error of a write rather than going on. Each test document is
// rendered once to note the call path of its writes, then once per path not
// seen yet, refusing its first write.
func TestRefusedWriteStopsTheRender(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.mjml"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	paths := 0
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		template := string(content)
		var refuse []int
		record := &refusingWriter{record: func(write int, path string) {
			if !seen[path] {
				seen[path] = true
				refuse = append(refuse, write)
			}
		}}
		if err := renderRefusing(t, template, record); err != nil {
			continue // a document that does not render
		}
		for _, write := range refuse {
			paths++
			err := renderRefusing(t, template, &refusingWriter{refuse: write})
			if !errors.Is(err, errWriteRefused) {
				t.Errorf("%s: write %d refused, the render returned %v", file, write, err)
			}
		}
	}
	t.Logf("%d call paths refused", paths)
}
