package mjml_test

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/Notifuse/safe-mjml/mjml"
)

// concurrentValidationTemplates set attributes of every type (color, unit,
// enum, boolean, integer), valid and invalid, including the forms only
// mjml-core's expressions decide (rgb() inside a value, glued lengths), so
// that the renders build the checker table and compile those expressions
// while others read them.
var concurrentValidationTemplates = []string{
	`<mjml><mj-body><mj-section background-color="x rgb(1,2,3)" padding="1px2px"><mj-column><mj-text color="#abc" align="bogus" font-size="10">x</mj-text><mj-button background-color="RGBA(1, 2, 3, 0.5)" inner-padding="1px 2px 3px 4px 5px">b</mj-button></mj-column></mj-section></mj-body></mjml>`,
	`<mjml><mj-head><mj-attributes><mj-text padding="1em" /><mj-all font-size="x" /></mj-attributes></mj-head><mj-body><mj-section full-width="nope"><mj-column><mj-table cellpadding="a4b" cellspacing="" /><mj-image src="a.png" fluid-on-mobile="yes" width="100" /></mj-column></mj-section></mj-body></mjml>`,
	`<mjml><mj-body><mj-section><mj-column><mj-divider border-color="Red" border-width="-1px" /><mj-spacer height="auto" /><mj-text letter-spacing="-2em" line-height="1.5">y</mj-text></mj-column></mj-section></mj-body></mjml>`,
}

// TestConcurrentValidation renders in parallel, from a fresh process so that
// the first renders build the checker table and compile the expressions
// while the others use them. Run under -race (CI does), it fails if any of
// them is written after its first use without synchronization.
func TestConcurrentValidation(t *testing.T) {
	if os.Getenv("SAFE_MJML_CONCURRENT_VALIDATION") == "1" {
		renderInParallel(t)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestConcurrentValidation$", "-test.count=1")
	cmd.Env = append(os.Environ(), "SAFE_MJML_CONCURRENT_VALIDATION=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("parallel renders in a fresh process: %v\n%s", err, out)
	}
}

func renderInParallel(t *testing.T) {
	want := make([]int, len(concurrentValidationTemplates))
	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	counts := make([][]int, 32)
	for g := range counts {
		counts[g] = make([]int, len(concurrentValidationTemplates))
		wg.Add(1)
		go func() {
			defer wg.Done()
			start.Wait()
			for i := range 20 {
				n := (g + i) % len(concurrentValidationTemplates)
				result, err := mjml.Compile(concurrentValidationTemplates[n])
				if err != nil {
					t.Error(err)
					return
				}
				counts[g][n] = len(result.Warnings)
			}
		}()
	}
	start.Done()
	wg.Wait()
	for n, tpl := range concurrentValidationTemplates {
		result, err := mjml.Compile(tpl)
		if err != nil {
			t.Fatal(err)
		}
		want[n] = len(result.Warnings)
		if want[n] == 0 || !strings.Contains(warningsText(result.Warnings), "has invalid value") {
			t.Errorf("template %d: no value warning, the test checks nothing", n)
		}
	}
	for g := range counts {
		for n, c := range counts[g] {
			if c != 0 && c != want[n] {
				t.Errorf("goroutine %d, template %d: %d warnings, %d alone", g, n, c, want[n])
			}
		}
	}
}

func warningsText(ws []mjml.Warning) string {
	var b strings.Builder
	for _, w := range ws {
		b.WriteString(w.Message + "\n")
	}
	return b.String()
}
