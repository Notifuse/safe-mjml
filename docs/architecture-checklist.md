# Architecture checklist

Checks to run when reviewing safe-mjml's architecture, or before a release. Each one comes with a command, and, in brackets, what it found here so far.

The tests and benchmarks of the repository only measure what the repository itself sees: MJML parity, ns/op, allocations. Several costs are paid elsewhere: by the program that imports safe-mjml, on another CPU, or on input nobody wrote a fixture for. Measure each item instead of reasoning about it, and look from the importer's side first.

When asking someone (or another agent) to review, give them these angles rather than your own hypotheses: hypotheses narrow the search to what you already suspect.

## 1. Consumer: what the library costs whoever imports it

- **Binary footprint.** Build a minimal consumer, with and without a reflective method call, and compare what the linker keeps. As soon as `reflect.Value.MethodByName` is reachable (`text/template` and `html/template` use it for `{{.Method}}`), the linker keeps every exported method of every type that may reach it, including methods promoted from embedded structs. [2026-11-11, v0.16.0: 966 safe-mjml symbols, 423 KB of code, without a reflective call; 2,418 symbols, 560 KB, with one, largely the exported `BaseComponent` methods promoted into the component types. After step 3, which unexported them: 953 symbols, 425 KB; 977 symbols, 428 KB with a reflective call. 2026-11-12, after the move to internal/ and the minimal API: 1,257 symbols, 389 KB; 1,280 symbols, 392 KB with a reflective call, against 1,260 and 386 KB, 1,282 and 389 KB just before.]

  ```go
  // probe/tmpl/main.go: delete the template lines for the baseline build
  package main

  import (
  	"fmt"
  	"os"
  	"text/template"

  	"github.com/Notifuse/safe-mjml/mjml"
  )

  type data struct{ Name string }

  func (d data) Upper() string { return d.Name }

  func main() {
  	html, err := mjml.Render(os.Args[1])
  	fmt.Println(len(html), err)
  	t := template.Must(template.New("x").Parse("{{.Upper}}"))
  	_ = t.Execute(os.Stdout, data{Name: "x"})
  }
  ```

  ```bash
  go build -o /tmp/probe ./probe/tmpl
  go tool nm -size /tmp/probe | grep -c 'Notifuse/safe-mjml'
  go tool nm -size /tmp/probe | grep 'Notifuse/safe-mjml' | awk '{s+=$2} END {print s}'
  ```

- **Exported API surface.** Everything exported is a promise to consumers and, under reflection, weight in their binary. Count it (`go doc -short ./mjml`, the only public package; `go doc -all ./internal/components` for the weight under reflection), check which exported identifiers are used outside their package (`git grep`), and compare tags with `gorelease` before a release. [After step 3, `BaseComponent` exports only the 7 methods of the `Component` interface. Since the move to internal/: `mjml` alone, 16 lines of `go doc -short` against 26; since cmd/safe-mjml/command moved under internal too, no other importable package, which `TestMJMLIsTheOnlyImportablePackage` checks.]
- **Dependencies.** `go list -deps ./mjml | grep '\.'` lists third-party packages pulled by the library; test-only dependencies must not leak into the module requirements. [Since the juice port: `golang.org/x/net/html` and `html/atom`, the HTML tokenizer of internal/juicer, which also sets the go directive to 1.26; nothing else, and no crypto package since premailer's RemoveIDs was removed.]
- **Startup cost and global state.** Package-level variables, `init()` functions, regexps and maps built at load time: every consumer pays for them, even if it never renders. Run `GODEBUG=inittrace=1` on a consumer. [14 package-level variables, including the test-mode ID lists and `DefaultBodyWidth`.]
- **Goroutines.** Any goroutine the library starts must have a documented lifecycle; use `go.uber.org/goleak` in tests. [One per `mjml.Cache`, its cleanup, from `NewCache` to `Close` (`internal/astcache`).]
- **Concurrency contract.** Render different templates in parallel under `go test -race`, with and without the cache.
- **Cancellation and limits.** Can a consumer bound the time or memory of one render? [There is no `context.Context` in the API. Notifuse compiles templates written by its users.]
- **Hostile input.** Deep nesting, very large text or attributes, many components, then fuzzing (`go test -fuzz` on the parser and on `Render`). Watch for super-linear time or memory. [20,000 nested `<span>`, 5 MB of text, 20,000 `mj-text` and a 1 MB attribute: all under 0.1 s. No fuzz test yet.]
- **Same output on every CPU.** Run the tests on arm64 and amd64 (`GOARCH=amd64 go test ./...` runs through Rosetta on Apple Silicon). Go may fuse a multiply and an add (FMA on arm64) even across statements; an explicit `float64(...)` conversion is what prevents it. [Found the VML rounding bug: 34% printed `-0.15999999999999998` instead of MJML's `-0.16`.]
- **Same output from every entry point.** `Render`, `Compile` (with and without a cache) and `Parse` with `Document.Render` must agree (`TestPublicEntryPointsAgree`). [Before the fix, 203 of 240 documents differed.]

## 2. Code structure

- **Hotspots.** Rank files by complexity and by churn (`gocognit -top 20 .`, `git log --format= --name-only | sort | uniq -c | sort -rn | head`). Files high on both lists are refactor candidates. [`section.go` (`Render` was about 650 lines), `wrapper.go`.]
- **The same type switch in many places.** Count `case *MJ...Component` per function. Each copy is a place to forget when a new nesting or component is supported. [The component tree is built and walked level by level, so content in some nestings is dropped silently (P1 in `issues/architecture-foundations-refactor.md`).]
- **Logic that must stay in sync by hand.** Two passes that resolve the same thing (rendering and `render_metadata.go`), or several resolvers for the same concept. [Several attribute resolution chains (P2).]
- **Output patched after the fact.** `strings.Replace` or a regexp on the final HTML is fixing the output instead of its source. [`normalizeGroupColumnClassOrder`, removed with P5.]
- **Mutable state during rendering.** Fields written in `Render`, render options copied or mutated by parents, passes whose order matters. [Outlook conditional chaining through `PendingMSOSectionClose` (P3), attributes written during render (P6).]
- **Dead code.** `deadcode ./...` (`golang.org/x/tools/cmd/deadcode`) and `staticcheck ./...` (U1000). [`InsideHero` was never read (removed at step 2); unreachable continuation branches in the full-width wrapper.]
- **Duplication.** `dupl -t 50 ./mjml/...`. [The two wrapper render paths, the two social-element render modes.]
- **Magic strings.** CSS properties and MJML attributes must come from `internal/constants` (AGENTS.md).

## 3. The test oracle: do the tests catch a change?

- **Strictness of the comparison.** Check what the comparison ignores. [CSS was compared as sorted characters, so swapped rules or values passed (P7). Until v0.16.0, inline styles were compared as sorted maps, so a shorthand written after its longhand passed: the accordion lost its padding sides and mj-hero its background position; styles are now compared by the declarations that take effect.]
- **The parser does not hide missing tags.** The parsed DOM repairs a missing `<tr>` or `</td>`: compare the raw tag sequence too. [A review of the oracle found that removing the `<tr>` and closing tags of mj-text failed only 10 reference comparisons out of 397 changed documents; the raw sequence then found that safe-mjml drops the template's HTML comments.]
- **Branches on enum attributes take MJML's default branch.** For every `if` on an attribute MJML types as `enum(...)`, the branch taken for any other value (empty, invalid) must be the one MJML takes: mjml-social tests `mode === 'horizontal'`, so `mode == "vertical"` is wrong even though both agree on the two valid values. List them with `git grep -nE '== *"(left|right|center|top|bottom|middle|horizontal|vertical|ltr|rtl|hidden|visible|full-width)"' internal/components` and compare each with the MJML source. The `enum-*` cases of the matrix set every value, the empty one, an invalid one, and the empty one over each lower layer. [Step 3: three such readers (social mode, accordion icon-position, the css-class join) failed only for an empty value over a lower layer.]
- **Every attribute in every layer, before a release.** `SWEEP=1 tools/release-diff.sh` adds the layer sweep (`tools/mjml-reference/layer-sweep.mjs`, about 6,800 documents: each attribute of each component in the element, mj-class, the mj-attributes tag, mj-all, the parent and the grandparent, with values and empty values). `TestLayerSweepParity` checks the same documents against MJML itself, not only against the last release, on every CI run: a difference not listed in `testdata/sweep-known-failures.txt` fails. [On step 3 it found that an empty `src` over a class or tag value made `mjml.Render` fail, and that an empty `background-url` over a lower layer dropped the background MJML writes.]
- **Every document is checked.** `TestEveryDocumentIsListed`: a fixture in no list is never compared. [Before v0.16.0, `validation-warnings` had a reference that no test compared.]
- **Mutation testing.** `gremlins unleash ./mjml/...`: every mutant that survives is behaviour no test checks.
- **Generated cases rather than hand-written fixtures.** MJML lists the allowed parent and child tags in `tools/mjml-reference/node_modules/mjml-preset-core/lib/dependencies.js`, and each component's allowed attributes in its `allowedAttributes`. Generate every parent/child pair, and every attribute set through the element, `mj-class`, `mj-attributes` and `mj-all`, render each case with both implementations (`tools/mjml-reference`), and compare. [The `mj-hero` and `mj-wrapper > mj-hero` content losses, and the ignored accordion attributes, are the kind of bug this finds.] [2026-11-18: `tools/mjml-reference/placement.mjs` and `TestPlacement` generate the parent/child pairs: 484 documents, 22 components in 11 containers, alone and after a sibling. Against v1.0.0 and MJML 4.18 they found 270 differences, all misplaced children (dropped by mj-group, mj-navbar, mj-social and the accordion, or an mj-group outside a section writing an Outlook cell), which v1.1.0 had fixed without this corpus; against v1.1.0 and MJML 5.4.1, 22 differ, all by MJML's `NaN`.]
- **Non-round values and empty attributes.** Check that the cases do not all use round values: with a 600px body and 50% columns, a width truncated or rounded at the wrong step gives the right output, and an absent attribute is not an empty one in MJML. [2026-11-11: the step 2 refactor truncated a group's 402.67px width to 402px and dropped the gutter of a column with `padding=""`; no hand-written case caught it. The attribute matrix, `tools/mjml-reference/matrix.mjs` and `TestMJMLMatrix`, now generates these cases: 261 documents, 183 differed from MJML in v0.15.1.]
- **Invariants on every document.** `TestRenderInvariants` (content visible outside Outlook, finite widths within the body, balanced conditional comments, no VML outside them, deterministic render, no error or panic) and `FuzzRender`. [The visibility invariant catches the nested-section content loss fixed in v0.15.0; the VML one, a section with a background-url in a wrapper writing its VML for every client, in v0.14 to v0.16.]
- **No regression against the previous release.** `tools/release-diff.sh` before every tag: no fragment of an MJML reference that the previous release wrote may be lost, even in a document that already differed, and a document that matched MJML must still match it. [This is how a review found the step 2 regressions, by hand. The review of v0.16.0 showed that comparing whole documents only left 209 of 560 unchecked; a review of the oracle, that counting fragments misses what moves, hence the token distance to the reference.]
- **References up to date.** `node tools/mjml-reference/regenerate.mjs --check` and `node tools/mjml-reference/matrix.mjs --check` must report no difference.

## 4. Runtime performance (beyond ns/op)

- CPU and memory profiles on the benchmarks (`go test -bench . -cpuprofile cpu.out -memprofile mem.out ./mjml`, then `go tool pprof`).
- Escape analysis on hot paths (`go build -gcflags=-m ./mjml/... 2>&1 | grep escapes`).
- Scaling with input size: 10, 100, 1000 sections must stay linear. [Linear: about 3.5 ms for 100 sections, 35 ms for 1000.]
- Contention: `b.RunParallel` on cached and uncached renders.
- Memory retention: what grows over a process lifetime. [The AST cache has no size limit.]

## 5. Evolution and operations

- **CI runs.** Check that GitHub Actions actually run on pushes and pull requests (`gh run list`). `.github/workflows/test.yml` runs vet, the tests and the race detector on amd64 and arm64, the 32-bit builds, the MJML reference checks and the release gate with the layer sweep. [On 2026-11-11, no run had ever happened on the fork; the workflow tested Go versions older than go.mod.]
- **Documentation drift.** AGENTS.md files, `mjml/testdata/AGENTS.md`, README and `docs/` must match how things work now.
- **Reproducible tooling.** Reference generation pins its MJML version (`tools/mjml-reference/package.json`).
- **Release notes and API diff** for every tag, after `tools/release-diff.sh` passes.
