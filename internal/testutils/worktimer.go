package testutils

import "time"

// WorkTimer measures the work a test does in the CPU time of its process,
// user and system, where the platform reports it (processCPUTime), and in
// wall-clock time elsewhere. A test that bounds how long a hostile template
// takes to be refused compares this with its limit: go test ./... runs the
// packages side by side, each in a process of its own, so on a loaded runner
// the wall clock of a test grows with the other packages, while its CPU time
// grows only with its own work. On GitHub's amd64 runner, a refusal that
// takes 2 to 3 s of CPU took 10 and 20 s of wall clock.
type WorkTimer struct {
	wall time.Time
	cpu  time.Duration
	ok   bool
}

// StartWorkTimer starts a WorkTimer.
func StartWorkTimer() WorkTimer {
	cpu, ok := processCPUTime()
	return WorkTimer{wall: time.Now(), cpu: cpu, ok: ok}
}

// Elapsed returns the CPU time the process spent since the start, or the
// wall-clock time where the CPU time is not reported.
func (w WorkTimer) Elapsed() time.Duration {
	if w.ok {
		if cpu, ok := processCPUTime(); ok {
			return cpu - w.cpu
		}
	}
	return time.Since(w.wall)
}
