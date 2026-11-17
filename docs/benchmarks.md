# Performance

safe-mjml is measured against MJML 5.4.1, the JavaScript implementation, on GitHub's Linux arm64 runner: a machine anyone can rerun the measure on, and that no other work shares.

## safe-mjml against MJML 5.4.1

`tools/bench-vs-mjml.sh` measures, for two templates:
- the [Austin template of mjml.io](../mjml/testdata/austin-layout-from-mjml-io.mjml), which has inline `mj-style` rules, so safe-mjml runs its CSS inliner on it as MJML runs juice;
- a [Notifuse email](../mjml/testdata/notifuse-full.mjml), as Notifuse's builder writes it, without inline rules.

It takes three measures:
- **One render, in process, warm.** For safe-mjml, Go's benchmark `BenchmarkReadmeTemplates`. For MJML, the mean of 500 renders in Node.js, after 50 to warm up.
- **50 compilations through the CLI**, one process each, with the output discarded. This includes the start of each process.
- **The peak memory of the CLI** for one compilation, read as the maximum resident set size.

Measured on 2026-11-17 at f30ee46, on Linux aarch64 with 4 CPUs, Go 1.26.0 and Node.js 24.22.2:

| Template | | safe-mjml | MJML 5.4.1 |
|---|---|---|---|
| `austin-layout-from-mjml-io` | One render, in process, warm | 1.47 ms | 7.7 ms |
| | 50 compilations through the CLI | 0.25 s | 26.05 s |
| | Peak memory of the CLI | 10 MB | 153 MB |
| `notifuse-full` | One render, in process, warm | 0.70 ms | 4.8 ms |
| | 50 compilations through the CLI | 0.22 s | 25.10 s |
| | Peak memory of the CLI | 10 MB | 145 MB |

On 2026-11-14, at a67c061, MJML 4.18 took 7.7 ms and 4.7 ms for one render, 22.21 s and 21.34 s for 50 compilations, and 134 MB and 129 MB of memory; safe-mjml then took 1.47 ms and 0.70 ms, 0.26 s and 0.22 s, and 10 MB.

An inline `mj-style` costs safe-mjml what it costs MJML: the document is read and written again as juice does. On this runner the Austin template took 0.95 ms before the inliner was embedded (224d4bf, Go 1.24), when safe-mjml applied class rules only.

To rerun it, start the workflow "Benchmarks against MJML" (`.github/workflows/bench.yml`) by hand, or push a branch named `bench/…`. On another machine:

```bash
(cd tools/mjml-reference && npm ci)
tools/bench-vs-mjml.sh
```

A laptop that is doing other work gives numbers 2 to 3 times slower, and not in the same ratio for both programs: measure on a quiet machine.

## Go benchmarks and the release gate

`./bench.sh` runs the Go benchmarks of `mjml/benchmark_test.go`, which render generated templates of 10, 100 and 1,000 sections.

`tools/release-diff.sh` runs four of them against the previous release, as part of the release gate. It fails a pull request that makes one of them more than 15% slower, unless `tools/accepted-slowdowns.txt` lists that benchmark with the reason.
