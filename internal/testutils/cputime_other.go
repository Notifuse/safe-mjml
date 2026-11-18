//go:build !unix

package testutils

import "time"

// processCPUTime reports false: WorkTimer then measures the wall clock.
func processCPUTime() (time.Duration, bool) {
	return 0, false
}
