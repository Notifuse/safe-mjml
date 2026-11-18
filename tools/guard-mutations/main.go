// Command guard-mutations removes each guard of safe-mjml, one at a time, and
// checks that the tests fail without it: a guard that no test isolates can be
// lost without anyone noticing.
//
// Run from the root of the repository:
//
//	go run ./tools/guard-mutations            # every guard
//	go run ./tools/guard-mutations NAME...    # the guards whose name contains NAME
//
// Each mutation replaces a piece of code that must occur exactly once in its
// file; a guard that moved fails the run, so that the list is kept up to
// date. The mutated file is given to go test with -overlay: nothing is
// written in the repository.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// guard is one guard and the mutation that removes it: old replaced by new
// in file.
type guard struct {
	name string
	file string
	old  string
	new  string
}

var guards = []guard{
	// Size and nesting of the template and of the HTML
	{"nesting of elements", "internal/parser/parser.go",
		"if depth >= MaxNesting {",
		"if false {"},
	{"size of the HTML as it is written", "mjml/render.go",
		"if d.err == nil && d.max > 0 && len(d.b)+len(s) > d.max {",
		"if false {"},
	{"size of the HTML after inline mj-style", "mjml/render.go",
		"if maxOutput > 0 && len(juiced) > maxOutput {",
		"if false {"},
	{"size of the HTML of mj-html-attributes", "internal/juicer/attributes.go",
		"if o.maxOutputSize > 0 && size > o.maxOutputSize {",
		"if false {"},

	// Work and memory of inline mj-style and mj-html-attributes
	{"step budget", "internal/cascadia/budget.go",
		"b.left -= int64(n)",
		"_ = n"},
	{"bytes of selector", "internal/juicer/limits.go",
		"*o.selectorBytes += n",
		"_ = n"},
	{"declarations an element gets cost steps", "internal/juicer/inline.go",
		"if !o.budget.Spend(declarationSteps*len(r.decls) + valueSteps(r.valueBytes)) {",
		"if false {"},
	{"attributes set cost steps", "internal/juicer/attributes.go",
		"if !o.budget.Spend(declarationSteps + len(n.Attr) + valueSteps(len(a.Key)+len(a.Val))) {",
		"if false {"},
	{"copies of template tags cost steps", "internal/juicer/codeblocks.go",
		"if extra > 0 && !budget.Spend(valueSteps(extra)) {",
		"if false {"},
	{"positional pseudo-classes cost steps", "internal/juicer/attributes.go",
		"if !s.budget.Spend(filterSteps + max(k, 0)) {",
		"if false {"},
	{"arguments of :not() cost steps", "internal/juicer/attributes.go",
		"if !s.budget.Spend(argumentSteps) {",
		"if false {"},

	// Nesting of documents, stylesheets and selectors
	{"element depth of the inline and attribute passes", "internal/juicer/htmlparser2/htmlparser2.go",
		"if maxDepth > 0 && len(stack) >= maxDepth {",
		"if false && maxDepth > 0 {"},
	{"stylesheet depth", "internal/juicer/limits.go",
		"if cssLimit > 0 && deepestCSS > cssLimit {",
		"if false {"},
	{"selector nesting of mj-html-attributes", "internal/juicer/selector/parse.go",
		"if p.limit > 0 && p.nest > p.limit {",
		"if false {"},
	{"stray parentheses of mj-html-attributes", "internal/juicer/selector/parse.go",
		"if p.limit > 0 && p.nest+parens > p.limit {",
		"if false {"},
	{"positional pseudo-classes of a path", "internal/juicer/attributes.go",
		"if positionals(toks) > maxPositionals {",
		"if false {"},
	{"selector nesting of cascadia", "internal/cascadia/parser.go",
		"if p.depth > maxParseDepth {",
		"if false {"},
	{"expansion of :is() lists", "internal/juicer/selector/expand.go",
		"if len(next)+len(alts) > maxExpansion {",
		"if false {"},
	{"substitutions of CSS variables", "internal/juicer/variables.go",
		"*budget--",
		"_ = budget"},

	// Panics and escaping
	{"a panic is returned as an error", "mjml/error.go",
		"\tr := recover()\n\tif r == nil {",
		"\tvar r any\n\tif r == nil {"},
	{"a double quote is escaped in an attribute", "internal/html/tag.go",
		"return strings.ReplaceAll(value, `\"`, \"&quot;\")",
		"return value"},
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	tmp, err := os.MkdirTemp("", "guard-mutations-")
	if err != nil {
		fail(err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	bad := 0
	for i, g := range guards {
		if !selected(g.name, os.Args[1:]) {
			continue
		}
		verdict := run(root, tmp, i, g)
		fmt.Printf("%-50s %s\n", g.name, verdict)
		if !strings.HasPrefix(verdict, "killed") {
			bad++
		}
	}
	if bad > 0 {
		fmt.Printf("\n%d guard(s) not isolated by a test\n", bad)
		os.Exit(1)
	}
}

// run applies the mutation of g and runs the tests, and says what happened.
func run(root, tmp string, i int, g guard) string {
	path := filepath.Join(root, g.file)
	src, err := os.ReadFile(path)
	if err != nil {
		return "error: " + err.Error()
	}
	if n := bytes.Count(src, []byte(g.old)); n != 1 {
		return fmt.Sprintf("not found: the code occurs %d times in %s, update the guard", n, g.file)
	}
	mutated := filepath.Join(tmp, fmt.Sprintf("%d_%s", i, filepath.Base(path)))
	if err := os.WriteFile(mutated, bytes.Replace(src, []byte(g.old), []byte(g.new), 1), 0o644); err != nil {
		return "error: " + err.Error()
	}
	overlay, _ := json.Marshal(map[string]map[string]string{"Replace": {path: mutated}})
	overlayFile := filepath.Join(tmp, fmt.Sprintf("%d.json", i))
	if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
		return "error: " + err.Error()
	}
	cmd := exec.Command("go", "test", "-overlay", overlayFile, "-count=1", "-timeout", "600s", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	switch {
	case err == nil:
		return "SURVIVED: no test fails without this guard"
	case bytes.Contains(out, []byte("[build failed]")) || bytes.Contains(out, []byte("[setup failed]")):
		return "error: the mutation does not compile\n" + string(out)
	case bytes.Contains(out, []byte("--- FAIL")):
		return "killed"
	case bytes.Contains(out, []byte("panic: test timed out")):
		return "killed (a test hangs)"
	case bytes.Contains(out, []byte("\nFAIL\t")):
		return "killed (the tests crash)"
	}
	return "error: go test failed without a failing test\n" + string(out)
}

func selected(name string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if strings.Contains(name, f) {
			return true
		}
	}
	return false
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
