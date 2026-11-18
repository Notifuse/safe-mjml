//go:build linux && !race

package mjml

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

// memoryFamilyEnv names the family a child process of
// TestRenderMemoryIsBounded renders; "-" builds the template of the first
// family without rendering it, for the memory of the process itself.
const memoryFamilyEnv = "SAFE_MJML_MEMORY_FAMILY"

// memoryTemplateSize is the size each family is grown to: a few MB, as a
// template sent to a server can be.
const memoryTemplateSize = 4 << 20

// maxBytesPerTemplateByte bounds the peak memory of a render, past what
// the process holds before it, per byte of template.
const maxBytesPerTemplateByte = 150

// TestRenderMemoryIsBounded grows each scaling family to a few MB and
// renders it in a process of its own, whose peak memory (the most it held,
// garbage included) must stay under maxBytesPerTemplateByte per byte of
// template, whether the template renders or is refused. Time grows
// linearly in every family (TestRenderScalesLinearly), but memory can be
// linear with a large factor, and a few MB of template then take gigabytes.
func TestRenderMemoryIsBounded(t *testing.T) {
	if name, ok := os.LookupEnv(memoryFamilyEnv); ok {
		renderMemoryFamily(name)
		return
	}
	if testing.Short() {
		t.Skip("renders each family in a process of its own")
	}
	base := childPeak(t, "-")
	for _, f := range scalingFamilies {
		t.Run(f.name, func(t *testing.T) {
			size := len(growFamily(f))
			peak := childPeak(t, f.name)
			per := float64(peak-base) / float64(size)
			if per > maxBytesPerTemplateByte {
				t.Errorf("%d bytes of template: %d MB at the peak, %.0f bytes per byte, over %d", size, peak>>20, per, maxBytesPerTemplateByte)
			} else {
				t.Logf("%d bytes of template: %d MB at the peak, %.0f bytes per byte", size, peak>>20, per)
			}
		})
	}
}

// growFamily is the template of f at the first n, doubling from f.n, at
// which it holds memoryTemplateSize bytes.
func growFamily(f scalingFamily) string {
	n := f.n
	for len(f.build(n)) < memoryTemplateSize {
		n *= 2
	}
	return f.build(n)
}

// renderMemoryFamily is the child process: it renders the family name, or
// only builds the template of the first family for "-".
func renderMemoryFamily(name string) {
	if name == "-" {
		_ = len(growFamily(scalingFamilies[0]))
		return
	}
	for _, f := range scalingFamilies {
		if f.name == name {
			_, _ = Render(growFamily(f))
			return
		}
	}
	panic(fmt.Sprintf("no family %q", name))
}

// childPeak runs this test in a child process for the family name and
// returns its peak resident memory, in bytes.
func childPeak(t *testing.T, name string) int64 {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRenderMemoryIsBounded$", "-test.count=1")
	cmd.Env = append(os.Environ(), memoryFamilyEnv+"="+name)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, out)
	}
	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		t.Skip("no resource usage")
	}
	return int64(usage.Maxrss) << 10 // in KB on Linux
}
