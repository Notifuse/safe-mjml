package mjml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestMJMLMatrix renders the generated attribute matrix of testdata/matrix
// (tools/mjml-reference/matrix.mjs: one box attribute per document, on the
// tag, empty, through mj-class and through mj-attributes, with non-round
// dimensions) and compares each document with its MJML reference.
//
// The gaps listed in testdata/matrix/known-failures.txt ("name: reason") are
// skipped with their reason; one of them that starts matching MJML fails the
// test until it is removed from the list, so the list cannot go stale.
func TestMJMLMatrix(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "matrix", "*.mjml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no matrix documents (run node tools/mjml-reference/matrix.mjs): %v", err)
	}
	sort.Strings(files)
	known := readMatrixKnownFailures(t)
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(strings.TrimSuffix(file, ".mjml") + ".html")
			if err != nil {
				t.Fatal(err)
			}
			// With the IDs of the reference, as every comparison with
			// MJML: random carousel and navbar IDs would make every such
			// document differ.
			actual, err := Render(string(source), WithIDGenerator(referenceIDGenerator(string(expected))))
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			differences := referenceDifferences(string(expected), actual)
			reason, isKnown := known[name]
			switch {
			case isKnown && len(differences) == 0:
				t.Errorf("%s now matches MJML: remove it from testdata/matrix/known-failures.txt", name)
			case isKnown:
				t.Skipf("known failure: %s", reason)
			case len(differences) > 0:
				t.Errorf("differs from MJML 4.18:\n%s", strings.Join(differences, "\n\n"))
			}
		})
	}
	for name := range known {
		if _, err := os.Stat(filepath.Join("testdata", "matrix", name+".mjml")); err != nil {
			t.Errorf("known-failures.txt lists %s, which is not a matrix document", name)
		}
	}
}

func readMatrixKnownFailures(t *testing.T) map[string]string {
	t.Helper()
	known := map[string]string{}
	file, err := os.Open(filepath.Join("testdata", "matrix", "known-failures.txt"))
	if os.IsNotExist(err) {
		return known
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, reason, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("known-failures.txt: %q is not \"name: reason\"", line)
		}
		known[strings.TrimSpace(name)] = strings.TrimSpace(reason)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return known
}

// TestMJMLMatrixWarnings compares the warnings of each matrix document with
// those of MJML's soft validation, which matrix.mjs records in
// testdata/matrix/warnings.json: the same messages, in the same order.
func TestMJMLMatrixWarnings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "matrix", "warnings.json"))
	if err != nil {
		t.Fatalf("no MJML warnings (run node tools/mjml-reference/matrix.mjs): %v", err)
	}
	var want map[string][]string
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join("testdata", "matrix", "*.mjml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no matrix documents: %v", err)
	}
	seen := map[string]bool{}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mjml")
		seen[name] = true
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Compile(string(source))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		var got []string
		for _, w := range result.Warnings {
			got = append(got, fmt.Sprintf("line %d (%s): %s", w.Line, w.TagName, w.Message))
		}
		if strings.Join(got, "\n") != strings.Join(want[name], "\n") {
			t.Errorf("%s: warnings\n got: %q\nwant: %q", name, got, want[name])
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("warnings.json lists %s, which is not a matrix document", name)
		}
	}
}
