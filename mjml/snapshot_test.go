package mjml

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// updateSnapshots rewrites testdata/snapshots from the current output:
//
//	go test ./mjml -run TestRenderSnapshots -update
var updateSnapshots = flag.Bool("update", false, "rewrite testdata/snapshots with the current output of Render")

const snapshotDir = "testdata/snapshots"

// TestRenderSnapshots compares the output of Render, byte for byte, with the
// snapshot of every test case in testdata, so that a refactoring that must not
// change the output can prove it did not. Unlike TestMJMLAgainstExpected, which
// compares with MJML's reference semantically, it catches any change at all,
// including in attribute order, whitespace and Outlook conditional comments.
//
// Snapshots record what safe-mjml produces, bugs included; they are not the
// expected output. When a change of output is intended, rewrite them with
// -update and review the diff of testdata/snapshots in the commit.
//
// testdata/snapshots/<name>.html holds the HTML and <name>.err the error
// Render returned, when it returned one. The render gets its carousel and
// navbar IDs from sequentialIDGenerator, so they read 0000000000000001,
// 0000000000000002… in order of first appearance.
func TestRenderSnapshots(t *testing.T) {
	// The documents of testdata, and those of the attribute matrix
	// (testdata/matrix, snapshots in testdata/snapshots/matrix), whose known
	// failures TestMJMLMatrix only skips: the snapshot is what shows that one
	// of them changed.
	for _, dir := range []string{"", "matrix"} {
		snapshotDocuments(t, filepath.Join("testdata", dir), filepath.Join(snapshotDir, dir), dir)
	}
}

func snapshotDocuments(t *testing.T, inputDir, outputDir, prefix string) {
	inputs, err := filepath.Glob(filepath.Join(inputDir, "*.mjml"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(inputs)
	if len(inputs) == 0 {
		t.Fatalf("no test case found in %s", inputDir)
	}

	if *updateSnapshots {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cases := map[string]bool{}
	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".mjml")
		htmlFile := filepath.Join(outputDir, name+".html")
		errFile := filepath.Join(outputDir, name+".err")
		cases[name] = true

		t.Run(filepath.Join(prefix, name), func(t *testing.T) {
			source, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			html, renderErr := Render(string(source), WithIDGenerator(sequentialIDGenerator()))
			errText := ""
			if renderErr != nil {
				errText = renderErr.Error() + "\n"
			}

			if *updateSnapshots {
				writeSnapshot(t, htmlFile, html)
				if errText != "" {
					writeSnapshot(t, errFile, errText)
				} else if err := os.Remove(errFile); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				return
			}

			compareSnapshot(t, htmlFile, html)
			if errText != "" {
				compareSnapshot(t, errFile, errText)
			} else if _, err := os.Stat(errFile); err == nil {
				t.Errorf("%s exists but Render returned no error; run with -update", errFile)
			}
		})
	}

	// Snapshot files whose test case no longer exists. (A stale .err file of
	// an existing case is reported by the case itself.) Other entries, such as
	// .DS_Store or the matrix directory, are not snapshots and are left alone.
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("%v (create the snapshots with: go test ./mjml -run TestRenderSnapshots -update)", err)
	}
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if !entry.Type().IsRegular() || (ext != ".html" && ext != ".err") || cases[strings.TrimSuffix(entry.Name(), ext)] {
			continue
		}
		path := filepath.Join(outputDir, entry.Name())
		if *updateSnapshots {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		t.Errorf("%s has no test case in %s; run with -update to remove it", path, inputDir)
	}
}

func writeSnapshot(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func compareSnapshot(t *testing.T, path, actual string) {
	t.Helper()
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (create it with: go test ./mjml -run TestRenderSnapshots -update)", err)
	}
	if string(expected) == actual {
		return
	}
	offset := firstDifference(string(expected), actual)
	t.Errorf("output differs from %s at byte %d\n  snapshot: %s\n  actual:   %s\n"+
		"If the change is intended, run: go test ./mjml -run TestRenderSnapshots -update",
		path, offset, excerpt(string(expected), offset), excerpt(actual, offset))
}

func firstDifference(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// excerpt returns the text around offset, on one line.
func excerpt(s string, offset int) string {
	start := max(0, offset-60)
	end := min(len(s), offset+60)
	return strings.ReplaceAll(fmt.Sprintf("…%s…", s[start:end]), "\n", `\n`)
}
