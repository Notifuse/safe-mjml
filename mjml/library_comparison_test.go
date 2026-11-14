package mjml

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLibraryComparison scores the outputs of Go MJML libraries against
// MJML 4.18's references, for the comparison in README.md and
// docs/library-comparison.md. tools/libcmp renders the documents with each
// library and minifies the outputs as MJML does; this test reads them from
// SAFE_MJML_LIBCMP_DIR and is skipped without it.
//
// Each output is compared with referenceDifferences, after two changes
// that do not affect how a client displays the email: the random IDs of
// the output are mapped onto the reference's, and an empty class
// attribute inside an Outlook conditional is dropped (MJML writes it, the
// minifier leaves it, other libraries do not write it). The second score
// also ignores the aria-roledescription and role attributes MJML added
// after 4.15, which the libraries embedding MJML 4.15 do not write.
func TestLibraryComparison(t *testing.T) {
	root := os.Getenv("SAFE_MJML_LIBCMP_DIR")
	if root == "" {
		t.Skip("set SAFE_MJML_LIBCMP_DIR to a directory written by tools/libcmp")
	}
	list, err := os.Open(filepath.Join(root, "docs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()
	var docs []string
	scanner := bufio.NewScanner(list)
	for scanner.Scan() {
		docs = append(docs, scanner.Text())
	}
	libraries, err := os.ReadDir(filepath.Join(root, "out-min"))
	if err != nil {
		t.Fatal(err)
	}
	for _, library := range libraries {
		matches, matchesWithoutAria, failures := 0, 0, 0
		for i, doc := range docs {
			reference, err := os.ReadFile(strings.TrimSuffix(doc, ".mjml") + ".html")
			if err != nil {
				t.Fatal(err)
			}
			output, err := os.ReadFile(filepath.Join(root, "out-min", library.Name(), fmt.Sprintf("%05d.html", i)))
			if err != nil {
				failures++
				continue
			}
			ref, out := comparableForLibraries(string(reference)), comparableForLibraries(mapIDs(string(reference), string(output)))
			if len(referenceDifferences(ref, out)) == 0 {
				matches++
			}
			if len(referenceDifferences(withoutAria(ref), withoutAria(out))) == 0 {
				matchesWithoutAria++
			}
		}
		fmt.Printf("%-12s matches MJML 4.18: %4d/%d (%.1f%%), without the post-4.15 aria attributes: %4d, render failures: %d\n",
			library.Name(), matches, len(docs), 100*float64(matches)/float64(len(docs)), matchesWithoutAria, failures)
	}
}

var (
	randomID          = regexp.MustCompile(`[0-9a-f]{16}`)
	outlookBlock      = regexp.MustCompile(`(?s)<!--\[if mso \| IE\]>.*?<!\[endif\]-->`)
	spaceBeforeTagEnd = regexp.MustCompile(`\s+>`)
)

// mapIDs replaces the random 16-hex IDs of output, in order of first
// appearance, with those of reference, when both have as many.
func mapIDs(reference, output string) string {
	refIDs, outIDs := distinctMatches(reference), distinctMatches(output)
	if len(refIDs) != len(outIDs) || len(outIDs) == 0 {
		return output
	}
	pairs := make([]string, 0, 2*len(outIDs))
	for i, id := range outIDs {
		pairs = append(pairs, id, refIDs[i])
	}
	return strings.NewReplacer(pairs...).Replace(output)
}

func distinctMatches(s string) []string {
	seen := map[string]bool{}
	var ids []string
	for _, id := range randomID.FindAllString(s, -1) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// comparableForLibraries drops the empty class attributes of the Outlook
// conditionals.
func comparableForLibraries(html string) string {
	return outlookBlock.ReplaceAllStringFunc(html, func(block string) string {
		return spaceBeforeTagEnd.ReplaceAllString(strings.ReplaceAll(block, ` class=""`, ""), ">")
	})
}

func withoutAria(html string) string {
	return strings.ReplaceAll(html, ` aria-roledescription="email" role="article"`, "")
}
