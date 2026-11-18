//go:build unix

package testutils

import (
	"syscall"
	"time"
)

// processCPUTime returns the user and system CPU time of the process.
func processCPUTime() (time.Duration, bool) {
	var r syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &r); err != nil {
		return 0, false
	}
	return time.Duration(r.Utime.Nano() + r.Stime.Nano()), true
}
