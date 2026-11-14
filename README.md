# safe-mjml - The best MJML engine in the world

A pure Go [MJML](https://mjml.io/) compiler: same HTML as MJML 4.18, 5 times faster, safe with untrusted templates.

It renders every email sent by [Notifuse](https://www.notifuse.com/), in production across hundreds of deployments.

![Tests](https://github.com/Notifuse/safe-mjml/actions/workflows/test.yml/badge.svg)
![MJML](https://img.shields.io/badge/MJML-4.18-orange)
![License](https://img.shields.io/badge/license-MIT-blue)

## Go MJML libraries compared

Measured on 7,969 documents on 2026-11-14 ([method](docs/library-comparison.md)):

| Library | Identical to MJML 4.18 | Render failures | Austin template |
|---|---|---|---|
| **safe-mjml** | **99.4%** | 0 | 1.25 ms |
| [krozhkov/mgml](https://github.com/krozhkov/mgml) | 82.4% | 14; crashes on concurrent renders | 4.13 ms |
| [preslavrachev/gomjml](https://github.com/preslavrachev/gomjml) | 18.3% | 601 | 0.59 ms |
| [julez-dev/mjmlgo](https://github.com/julez-dev/mjmlgo) | 0.1% (27.4% ignoring the whitespace of Outlook comments) | 3,148 | 1.43 ms |
| [TcMits/mjml2html](https://github.com/TcMits/mjml2html) | 0%, runs MJML 4.15 | 0 | 178 ms |
| [codedninja/mrmlgo](https://github.com/codedninja/mrmlgo) | 0%, runs MRML through cgo | 14 | 0.28 ms |

[Boostport/mjml-go](https://github.com/Boostport/mjml-go) and its fork [handsomefox/mjml-go](https://github.com/handsomefox/mjml-go) are not listed: they run MJML 4.15 in WebAssembly, take 2.7 s per render, and fail on any template with a validation warning.

## Why safe-mjml

- **Same output as MJML.** Tested against MJML 4.18 on 8,513 documents. Every difference is documented.
- **Fast.** The Austin template from mjml.io renders in 1.5 ms, against 7.7 ms with MJML in Node.js.
- **No runtime to install.** Pure Go: no Node.js, no WebAssembly, no cgo. Dependencies: the standard library and `golang.org/x/net`.
- **Safe with untrusted templates.** A template cannot crash your program, exhaust its memory or CPU, or break out of an attribute. See [Safety](#safety).
- **Clear errors.** Every error and warning has the line and the tag of its cause.

## Install

```bash
go get github.com/Notifuse/safe-mjml
```

```go
import "github.com/Notifuse/safe-mjml/mjml"

html, err := mjml.Render(`<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>Hello World!</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`)
```

## CLI

```bash
go install github.com/Notifuse/safe-mjml/cmd/safe-mjml@latest

safe-mjml compile input.mjml -o output.html   # to a file
safe-mjml compile input.mjml -s               # to stdout
safe-mjml compile input.mjml -s --strict      # exit 1 on validation warnings
```

`--debug` adds `data-mj-debug-*` attributes that trace the HTML back to the MJML.

## API

```go
html, err := mjml.Render(template)     // the HTML
result, err := mjml.Compile(template)  // the HTML and the validation warnings

doc, err := mjml.Parse(template)       // parse once,
html, err = doc.Render()               // render many times, from any goroutine
```

Options:

```go
mjml.WithKeepComments(false)     // drop the template's HTML comments (kept by default, as in MJML)
mjml.WithIDGenerator(nextID)     // IDs for mj-carousel and mj-navbar, for reproducible output
mjml.WithCache(cache)            // reuse parsed templates
```

The cache is opt-in. Use it when the same templates are rendered many times:

```go
cache := mjml.NewCache(10 * time.Minute) // entries expire after this TTL
defer cache.Close()
```

It has no size limit: an entry takes about 8 times the size of its template.

### Warnings

Like MJML, safe-mjml renders templates with unknown tags, misplaced components or invalid attributes, and reports them as warnings:

```go
result, err := mjml.Compile(template)
for _, w := range result.Warnings {
	fmt.Printf("line %d (%s): %s\n", w.Line, w.TagName, w.Message)
}
```

The messages are MJML's. `w.Kind` tells them apart: `WarningUnknownTag`, `WarningUnsupportedTag`, `WarningMisplacedTag`, `WarningInvalidAttribute` or `WarningInvalidAttributeValue`.

### Errors

Every error is an `*mjml.Error`, with the line and tag of its cause in `Details`:

```go
var e *mjml.Error
if errors.As(err, &e) {
	for _, d := range e.Details {
		fmt.Printf("line %d (%s): %s\n", d.Line, d.TagName, d.Message)
	}
}
```

A template whose root is not `<mjml>`, or without `mj-body`, is an error, as in MJML. A panic inside safe-mjml is returned as an error too (`errors.Is(err, mjml.ErrInternal)`), with its stack in `e.Stack`.

## Safety

safe-mjml is built to render templates you do not control:

- A panic never reaches your program: it is returned as an error.
- Nesting depth is limited.
- Rendering time grows linearly with the template, and inline `mj-style` stops at a work limit.
- The HTML is limited to 16 MB.
- Attribute values are always escaped.
- Malformed templates are refused with their line, never rendered with parts missing.
- No file or network access, and no code execution.
- Renders are safe to run concurrently.

Each point is tested: see [docs/safety.md](docs/safety.md). safe-mjml does not sanitize the HTML you put in `mj-text`, `mj-raw` and other content tags, and does not limit the size of the template.

## Tests

The tests compare safe-mjml's output with MJML 4.18, which is pinned in `tools/mjml-reference`:

| Corpus | Documents | Identical to MJML |
|---|---|---|
| Test cases | 447 | 437 (the other 10 are explained in [docs/compatibility.md](docs/compatibility.md)) |
| Every enum and size attribute | 610 | 608 |
| Every attribute of every component, at every level | 6,915 | 6,872 |
| MJML's official templates | 25 | 25 |
| Templates with one author mistake | 516 | rendered as MJML does, or refused with their line |

Every pull request also runs fuzzing, 42 scaling tests on hostile templates, escaping tests, the race detector, and a release gate that compares every document with the previous release. Details per component: [docs/compatibility.md](docs/compatibility.md).

## Performance

On GitHub's Linux arm64 runner, with Node.js 24 for MJML ([method](docs/benchmarks.md)):

| | safe-mjml | MJML 4.18 |
|---|---|---|
| Austin template (inline `mj-style`) | 1.47 ms | 7.7 ms |
| A Notifuse email | 0.70 ms | 4.7 ms |
| 50 runs of the CLI | 0.26 s | 22.2 s |
| CLI memory | 10 MB | 134 MB |

## Differences from MJML

- Not supported: `mj-include`, `mj-html-attributes`, custom components, and MJML's `minify`, `beautify` and `.mjmlconfig` options. The output is compact HTML, close to `mjml --minify`.
- Stricter: malformed MJML, invalid characters, content after `</mjml>`, HTML over 16 MB and inline styles over the work limit are refused with an error.
- On purpose: no `NaN`, `Infinity` or negative widths; quotes in attributes are escaped; Liquid tags are kept intact by inline `mj-style`.
- A few rare selectors in inline `mj-style` are not applied.

The full list: [docs/differences-from-mjml.md](docs/differences-from-mjml.md). If you find another difference, please [open an issue](https://github.com/Notifuse/safe-mjml/issues) with the MJML that shows it.

## Contributing

```bash
go test ./...
go test -race ./...
```

Each test case is a pair of files in `mjml/testdata`: `name.mjml` and MJML's output `name.html`. `go run ./cmd/htmlcompare name --testdata-dir mjml/testdata` shows how safe-mjml's output differs. See [AGENTS.md](AGENTS.md) for the conventions.

## License

MIT. Copyright (c) 2026 Notifuse and (c) 2025 Preslav Rachev. See [LICENSE](LICENSE).
