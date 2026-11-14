package mjml_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestOptionIsOpaqueToOtherModules builds, in a module of its own, a program
// that recovers the configuration an Option sets by generic type inference.
// It must get a struct whose fields it cannot reach: the internal render
// options (debug tags, warning handler) have no public option.
func TestOptionIsOpaqueToOtherModules(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a separate module")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go tool: %v", err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}

	build := func(t *testing.T, body string) (string, error) {
		t.Helper()
		dir := t.TempDir()
		files := map[string]string{
			"go.mod": "module probe\n\ngo 1.24\n\nrequire github.com/Notifuse/safe-mjml v0.0.0\n\nreplace github.com/Notifuse/safe-mjml => " + root + "\n",
			"go.sum": string(sum),
			"main.go": `package main

import "github.com/Notifuse/safe-mjml/mjml"

// configOf recovers the configuration type of an option by inference.
func configOf[T any](o func(*T)) *T {
	c := new(T)
	o(c)
	return c
}

func main() {
	c := configOf(mjml.WithKeepComments(false))
	` + body + `
}
`,
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(goTool, "build", "-mod=mod", "-o", os.DevNull, ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOPROXY=off")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// The inference itself compiles: the check below is not vacuous.
	if out, err := build(t, "_ = c"); err != nil {
		t.Fatalf("the probe does not build: %v\n%s", err, out)
	}
	for _, field := range []string{"DebugTags = true", "WarningHandler = nil", "SkipComments = true"} {
		out, err := build(t, "c."+field)
		if err == nil {
			t.Errorf("another module sets c.%s through an mjml.Option", field)
			continue
		}
		name := strings.Fields(field)[0]
		if !strings.Contains(out, name+" undefined") {
			t.Errorf("c.%s: unexpected build failure:\n%s", field, out)
		}
	}
}

// TestMJMLIsTheOnlyImportablePackage lists the packages of the module: apart
// from commands, every package other modules could import must be mjml.
func TestMJMLIsTheOnlyImportablePackage(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go tool: %v", err)
	}
	cmd := exec.Command(goTool, "list", "-f", "{{.ImportPath}} {{.Name}}", "./...")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		path, name, _ := strings.Cut(line, " ")
		if name == "main" || path == "github.com/Notifuse/safe-mjml/mjml" ||
			strings.Contains(path+"/", "/internal/") {
			continue
		}
		t.Errorf("%s can be imported by other modules: only mjml should be", path)
	}
}
