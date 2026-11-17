# The forks of juicer and cascadia

safe-mjml carries two third-party Go libraries in its module, changed for what it needs. They are forks, not copies: updating them means porting the upstream commits by hand. This page records where each one starts, what safe-mjml changed, and how the upstream commits since were sorted.

| Directory | Upstream | License | Starts from |
|---|---|---|---|
| `internal/juicer` (with `htmlparser2`, `mensch`, `postcss`, `selector`) | [customerio/go-juicer](https://github.com/customerio/go-juicer), a Go port of juice | MIT, `internal/juicer/LICENSE` | `d1674ed` (2026-11-06), the latest commit when this page was written |
| `internal/cascadia` | [andybalholm/cascadia](https://github.com/andybalholm/cascadia) | BSD, `internal/cascadia/LICENSE` | `v1.3.5` (`caa0b27`, 2026-09-04), the version go-juicer requires |

The starting points are the upstream commits closest to the forks: the diff with each is smaller than with any other of the last 40 commits.

## Why they are forks

What safe-mjml changed is inside the matching and the parsing, where neither library has a point to plug it from outside:

- **MJML's output.** MJML 4.18 runs juice 10.0.1, which matches selectors with css-select 5.2.2. juicer gained the `Juice10` mode, and cascadia a css-select mode (`ParseForJuice`, `internal/cascadia/cssselect.go`): attribute values compared as css-select compares them, `:contains`, `:empty`, css-select's aliases (`:checked`, `:header`…).
- **Hostile templates.** cascadia counts the work of each match (`Budget`, `internal/cascadia/budget.go`) and juicer bounds it (`MaxMatchSteps`, `MaxStylesheetDepth`, `MaxDepth`), so that a stylesheet written to make matching slow costs a bounded time (see [safety.md](safety.md)).
- **Template tags.** juicer keeps Liquid's `{% %}` intact (`CodeBlocks`).
- **Removed.** The options safe-mjml does not pass went: `PreserveMediaQueries`, `ApplyWidthAttributes`, `RemoveIDs`, `XMLMode` and others. So do go-juicer's tests that need them or its 38 MB oracle corpus: `parity_test.go`, `fuzz_test.go`, `premailer_test.go` and the ones that run juice with Node.

safe-mjml calls juicer in one mode only, `Juice10`. The code of the other modes stays (postcss, juice 12's ignore comments, the browser matchers of cascadia): no render reaches it, and Codecov leaves both directories out of its figure.

## Upstream commits since the starting points

**go-juicer**: none since `d1674ed`.

**cascadia**: 20 commits after `v1.3.5`, sorted by whether they change code a render of safe-mjml reaches (the mjml tests, run with `-coverpkg`), and, for those, by what MJML 4.18 writes.

Not reached: they change the browser matchers, which the css-select mode replaces.

| Commit | Change |
|---|---|
| `6d90623` | Reject empty substring and token attribute selector values |
| `ff180d2` | Restrict case-insensitive attribute matching to ASCII |
| `dfb0d9e` | Match whitespace-only attribute substrings |
| `75b0f74` | Merge of the two above |
| `0baee59` | Stop language inheritance at explicit lang attributes |
| `0b0ff2d` | Match language tags without ASCII case distinctions |
| `f61e147` | Honour disabled optgroups in option selectors |
| `00f96eb` | Keep Unicode spaces visible to `:empty` |
| `8dee753` | Stop matching links with `:enabled` |
| `e1e2249` | Split class lists with a byte table (performance) |
| `a346454` | Escape the argument of `:contains` when serializing |

Reached: each was checked with a template rendered by MJML 4.18 and by safe-mjml.

| Commit | Change | Verdict |
|---|---|---|
| `8b5adc2` | Accept leading combinators in `:has()` | Done otherwise: juicer rewrites `:has(+ x)` and `:has(~ x)` as cascadia's `:hasnext(x)` and `:haslater(x)`, one nested call per combinator, and drops a `:not()` there, which css-select ignores (case `mj-style-inline-has-sibling`). |
| `154416a` | Refactor `:not` into its own type | Not needed: `:not(.a .b)` already matches as MJML's. |
| `d561140` | Zero-valued escapes become U+FFFD | Not needed: `.z\0 a` and `[data-z="\0"]` already match as MJML's. |
| `2a17570` | Respect namespaces in of-type selectors | Not needed for HTML: `p:first-of-type` already matches as MJML's. |
| `59d99d7` | Escape quoted attribute values when serializing | Not needed: `[title='a"b']` and `[title="a\\b"]` already match as MJML's. |
| `7609871` | Support `:is()` and `:where()` | **Not to port**: juice 10 matches nothing with them (case `mj-style-inline-css-select-common`). |
| `2233b19` | Accept the `s` flag in attribute selectors | Already in the css-select mode. |
| `70c8c86` | Support `:any-link` | Already in the css-select mode (case `mj-style-inline-css-select`). |
| `dde0a32` | Support `:nth-child(An+B of S)` | Not needed: `li:nth-child(2 of .x)` already matches as MJML's. |

## Updating a fork

1. List the upstream commits after the starting point above (`git log <start>..main`).
2. For each, find the functions it changes and whether a render of safe-mjml reaches them: `go test -coverpkg=./... -coverprofile=c.out ./mjml` and `go tool cover -func=c.out`. A change to unreached code only (another mode of juicer, a browser matcher of cascadia) can be left out.
3. For a change to reached code, write a probe template that uses it and compare MJML 4.18 with safe-mjml: `node differential.mjs <probes>` in `tools/mjml-reference` renders both and prints where they differ. Port the change only if MJML's output agrees with it. When it fixes a difference with MJML, the template becomes a case of `mjml/testdata`.
4. Port it, then run the tests, the snapshots and the layer sweep (the release gate of the CI).
5. Update the starting point and the tables of this page.
