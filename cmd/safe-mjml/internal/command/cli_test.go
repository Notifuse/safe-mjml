package command

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	validDoc   = "<mjml><mj-body><mj-section><mj-column><mj-text>hello</mj-text></mj-column></mj-section></mj-body></mjml>"
	warningDoc = "<mjml>\n<mj-body><mj-section><mj-column><mj-text foo=\"bar\">hello</mj-text></mj-column></mj-section></mj-body></mjml>"
)

// run executes the CLI with args, as the safe-mjml binary does, and returns
// what it wrote on its standard and error outputs.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut strings.Builder
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errOut)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

// writeFile writes content in a file of a temporary directory and returns
// its path.
func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompileWritesTheHTMLToStdout(t *testing.T) {
	stdout, stderr, err := run(t, "compile", writeFile(t, "in.mjml", validDoc))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout, "<!doctype html>") || !strings.Contains(stdout, ">hello<") {
		t.Errorf("stdout is not the rendered document:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr: got %q, want nothing", stderr)
	}
}

func TestCompileWritesTheHTMLToTheOutputFile(t *testing.T) {
	output := filepath.Join(t.TempDir(), "out.html")
	stdout, _, err := run(t, "compile", writeFile(t, "in.mjml", validDoc), "-o", output)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Errorf("stdout: got %q, want nothing when -o is given", stdout)
	}
	html, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), ">hello<") {
		t.Errorf("the output file is not the rendered document:\n%s", html)
	}
}

func TestCompileDebugAddsTheDebugAttributes(t *testing.T) {
	input := writeFile(t, "in.mjml", validDoc)
	plain, _, err := run(t, "compile", input)
	if err != nil {
		t.Fatal(err)
	}
	debug, _, err := run(t, "compile", input, "--debug")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "data-mj-debug") {
		t.Error("debug attributes without --debug")
	}
	if !strings.Contains(debug, `data-mj-debug-text="true"`) {
		t.Errorf("no debug attribute with --debug:\n%s", debug)
	}
}

// Without --strict, a warning is printed and the HTML is still written.
func TestCompilePrintsWarningsAndSucceeds(t *testing.T) {
	input := writeFile(t, "in.mjml", warningDoc)
	stdout, stderr, err := run(t, "compile", input)
	if err != nil {
		t.Fatal(err)
	}
	if want := input + ":2: warning: (mj-text) Attribute foo is illegal\n"; stderr != want {
		t.Errorf("stderr: got %q, want %q", stderr, want)
	}
	if !strings.Contains(stdout, ">hello<") {
		t.Error("the document must still be rendered")
	}
}

// With --strict, a warning makes the command fail, after the HTML is written.
func TestCompileStrictFailsOnWarnings(t *testing.T) {
	input := writeFile(t, "in.mjml", warningDoc)
	output := filepath.Join(t.TempDir(), "out.html")
	_, stderr, err := run(t, "compile", input, "-o", output, "--strict")
	if !errors.Is(err, errReported) {
		t.Fatalf("got error %v, want the failure of --strict", err)
	}
	if !strings.HasSuffix(stderr, "1 validation warning(s), failing because of --strict\n") {
		t.Errorf("stderr: %q", stderr)
	}
	if _, err := os.Stat(output); err != nil {
		t.Errorf("the HTML must still be written: %v", err)
	}
}

func TestCompileStrictSucceedsWithoutWarnings(t *testing.T) {
	if _, stderr, err := run(t, "compile", writeFile(t, "in.mjml", validDoc), "--strict"); err != nil {
		t.Errorf("got error %v, stderr %q", err, stderr)
	}
}

func TestCompileFailures(t *testing.T) {
	dir := t.TempDir()
	for _, failure := range []struct {
		name   string
		args   []string
		stderr string
	}{
		{"missing input", []string{"compile", filepath.Join(dir, "missing.mjml")}, "Error reading file: "},
		{"render error", []string{"compile", writeFile(t, "in.mjml", "<mjml></mjml>")}, "Error rendering MJML: "},
		{"unwritable output", []string{"compile", writeFile(t, "in.mjml", validDoc), "-o", filepath.Join(dir, "no", "out.html")}, "Error writing output file: "},
	} {
		t.Run(failure.name, func(t *testing.T) {
			stdout, stderr, err := run(t, failure.args...)
			if !errors.Is(err, errReported) {
				t.Fatalf("got error %v, want a reported failure", err)
			}
			if !strings.HasPrefix(stderr, failure.stderr) || strings.Count(stderr, "Error") != 1 {
				t.Errorf("stderr: got %q, want one message starting with %q", stderr, failure.stderr)
			}
			if stdout != "" {
				t.Errorf("stdout: got %q, want nothing", stdout)
			}
		})
	}
}

// A usage error is left to cobra, which prints it with the usage (on the
// output set by SetOut here, on stderr in the binary).
func TestCompileWithoutInputIsAUsageError(t *testing.T) {
	stdout, stderr, err := run(t, "compile")
	if err == nil || errors.Is(err, errReported) {
		t.Fatalf("got error %v, want the usage error of cobra", err)
	}
	if stderr != "Error: accepts 1 arg(s), received 0\n" || !strings.Contains(stdout, "Usage:") {
		t.Errorf("got stdout %q, stderr %q, want the error once and the usage", stdout, stderr)
	}
}

// Given a file, the root command compiles it, with the flags of compile.
func TestRootCompilesAFile(t *testing.T) {
	input := writeFile(t, "in.mjml", warningDoc)
	output := filepath.Join(t.TempDir(), "out.html")
	stdout, stderr, err := run(t, input, "-o", output, "--strict", "--debug")
	if !errors.Is(err, errReported) {
		t.Fatalf("got error %v, want the failure of --strict", err)
	}
	if !strings.HasPrefix(stderr, input+":2: warning: ") || !strings.HasSuffix(stderr, "failing because of --strict\n") {
		t.Errorf("stderr: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout: got %q, want nothing when -o is given", stdout)
	}
	html, err := os.ReadFile(output)
	if err != nil || !strings.Contains(string(html), `data-mj-debug-text="true"`) {
		t.Errorf("the output file is not the debug rendering (%v):\n%s", err, html)
	}
}

// Two arguments are neither a command nor a file to compile.
func TestRootWithTwoArgumentsIsAUsageError(t *testing.T) {
	_, stderr, err := run(t, "compil", "in.mjml")
	if err == nil || errors.Is(err, errReported) || stderr != "Error: accepts at most 1 arg(s), received 2\n" {
		t.Errorf("got error %v, stderr %q", err, stderr)
	}
}

func TestVersion(t *testing.T) {
	stdout, _, err := run(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	// A test binary has no module version: Go records (devel).
	if !strings.HasPrefix(stdout, "safe-mjml ") || !strings.HasSuffix(stdout, " (MJML 4.18)\n") {
		t.Errorf("stdout: %q", stdout)
	}
}

func TestRootWithoutArgumentsPrintsTheHelp(t *testing.T) {
	stdout, _, err := run(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "safe-mjml is a native Go implementation") {
		t.Errorf("stdout is not the help:\n%s", stdout)
	}
}

func TestGoTestCommandArguments(t *testing.T) {
	for _, tc := range []struct {
		verbose bool
		pattern string
		want    []string
	}{
		{false, "", []string{"go", "test", "./mjml"}},
		{true, "basic", []string{"go", "test", "-v", "-run", "basic", "./mjml"}},
	} {
		if got := goTestCommand(tc.verbose, tc.pattern).Args; !slices.Equal(got, tc.want) {
			t.Errorf("goTestCommand(%v, %q): got %q, want %q", tc.verbose, tc.pattern, got, tc.want)
		}
	}
}

// The test command runs go test on the mjml package of the current
// directory: from the root of the repository, it succeeds.
func TestTestCommandRunsTheMJMLTests(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the tests of the mjml package")
	}
	t.Chdir(filepath.Join("..", "..", "..", ".."))
	stdout, stderr, err := run(t, "test", "-p", "^$")
	if err != nil {
		t.Fatalf("got error %v, stderr:\n%s", err, stderr)
	}
	if !strings.HasPrefix(stdout, "Running Go MJML tests...\n") || !strings.HasSuffix(stdout, "All tests passed!\n") {
		t.Errorf("stdout: %q", stdout)
	}
}

// Elsewhere, go test fails, and so does the command.
func TestTestCommandFailsOutsideTheRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	_, stderr, err := run(t, "test")
	if !errors.Is(err, errReported) {
		t.Fatalf("got error %v, want a reported failure", err)
	}
	if !strings.Contains(stderr, "Tests failed: ") {
		t.Errorf("stderr: %q", stderr)
	}
}
