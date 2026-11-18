# MJML Test Data Reference

## Purpose

This directory contains reference test cases for validating the safe-mjml compiler against the official MJML specification. Each test case consists of:

- **Input**: `.mjml` file containing MJML markup
- **Expected Output**: `.html` file containing the reference HTML output

The HTML files are generated with the official MJML implementation (version pinned in `tools/mjml-reference/package.json`, currently 5.4.1, its unminified render compacted by html-minifier 4.0.0, see `tools/mjml-reference/render.mjs`) and serve as the ground truth for verifying that our Go implementation produces spec-compliant output.

## File Structure

```
testdata/
├── basic.mjml          # Input MJML template
├── basic.html          # Expected HTML output
├── mj-button.mjml      # Button component test
├── mj-button.html      # Expected button HTML
└── ...                 # Additional test cases
```

## Snapshots

`snapshots/` holds, for every `.mjml` file here, the output of `mjml.Render()`
byte for byte (`<name>.html`, and `<name>.err` when Render returns an error),
checked by `TestRenderSnapshots` in `mjml/snapshot_test.go`. They record what
safe-mjml produces today, bugs included: they are a safety net for refactorings,
not a reference. The render gets its carousel and navbar IDs from a sequential
generator (`mjml.WithIDGenerator`), so they read `0000000000000001`,
`0000000000000002`… Rewrite them with
`go test ./mjml -run TestRenderSnapshots -update` only when a change of output
is intended.

## Carousel and navbar IDs

MJML gives each mj-carousel and each mj-navbar checkbox a random 16-hex-digit
ID. The integration tests render each case with `mjml.WithIDGenerator`, fed
with the IDs of its own reference in order of first appearance
(`referenceIDGenerator` in `mjml/integration_test.go`), so a case passes the
same way alone, filtered with `-run`, or shuffled.

## Integration with Tests

These test files are consumed by `mjml/integration_test.go` in the `TestMJMLAgainstExpected` function. The test:

1. Reads each `.mjml` file from the test case list
2. Compiles it using our Go implementation (`mjml.Render()`)
3. Compares the output against the corresponding `.html` file
4. Reports comprehensive differences if outputs don't match

## Test Case Categories

### Core Components
- `mjml.mjml/html` - Root document structure
- `mj-body.mjml/html` - Body container with attributes
- `mj-section.mjml/html` - Section layouts and styling
- `mj-column.mjml/html` - Column layouts and responsive behavior
- `mj-wrapper.mjml/html` - Wrapper components for grouping

### Content Components
- `mj-text.mjml/html` - Text rendering and styling
- `mj-button.mjml/html` - Button components with various attributes
- `mj-image.mjml/html` - Image handling and responsive behavior
- `mj-divider.mjml/html` - Divider styling and positioning
- `mj-social.mjml/html` - Social media icons and links

### Advanced Components
- `mj-group.mjml/html` - Column grouping and alignment
- `mj-hero.mjml/html` - Hero sections with background images
- `mj-navbar.mjml/html` - Navigation components
- `mj-accordion.mjml/html` - Collapsible content sections
- `mj-table.mjml/html` - Email-safe table rendering

### Layout Tests
- `complex-layout.mjml/html` - Multi-section complex layouts
- `austin-*.mjml/html` - Real-world template examples
- `wrapper-*.mjml/html` - Various wrapper configurations

## Debugging Failed Tests

When tests fail, the integration test provides detailed analysis:

### 1. Use htmlcompare Utility
```bash
# From project root
./bin/htmlcompare basic --testdata-dir mjml/testdata

# From testdata directory
../../bin/htmlcompare basic
```

### 2. Check Debug Files
Failed tests write temporary files for manual inspection:
- `/tmp/expected_{testname}.html` - Reference output
- `/tmp/actual_{testname}.html` - Our implementation output

### 3. Common Difference Types
- **DOM Structure**: Element count or nesting differences
- **CSS Properties**: Style attribute ordering or values
- **MSO Conditionals**: Outlook-specific conditional comments
- **VML Attributes**: Vector Markup Language attributes for email clients
- **HTML Entities**: Encoding differences (&amp; vs &)

## Adding New Test Cases

1. **Create MJML file**: Add `{testname}.mjml` with your test case
2. **Generate reference**: `cd tools/mjml-reference && npm install && node regenerate.mjs {testname}` writes `{testname}.html` (minified, as `mjml --minify` does). For an existing reference, carousel and navbar IDs are kept from it.
3. **Add to test list**: Include in `expectedCases` in `integration_test.go` (or in `knownFailures`, with the reason, if safe-mjml does not match MJML yet)
4. **Verify**: Run `go test -v ./mjml -run TestMJMLAgainstExpected`

### Rules for a new case

Reviews found regressions that every hand-written case missed, because they all used round values. A new case must:

- **Use non-round dimensions**: a body of 601px or 599px rather than 600px, columns of 33.33% or 200.9px, uneven paddings. A width truncated or rounded at the wrong step gives the same output with round values.
- **Cover the empty attribute** when the case is about an attribute: `padding=""` is not the same as no `padding` in MJML (the empty value replaces the default, and its presence alone can change the layout, as for a column's gutter).
- **Go through `mj-class` and `mj-attributes`** as well as the tag, when the case is about attribute resolution.

### When a fix changes what an existing test expects

Never rewrite a test so that it passes. A review found a regression that silently dropped the rest of an email; the only test with that structure (`<mj-text>Hello</mj-body>`, which expected an error) had been given another input in the same commit, so that it kept failing as before, and the new behaviour on the old input was never compared with MJML.

When a change of behaviour changes the expectation of an existing test:

- **keep its input**, and turn it into an MJML case (or a test of the new behaviour checked against MJML's render), so that the new expectation is MJML's, not the one the code happens to give;
- **say it in the commit message**: which test, what it expected, what it expects now, and how the new expectation was checked;
- **never** swap the input for one that still gives the old result, loosen an assertion, widen a tolerance, or add an entry to `knownFailures`, `invariantExceptions`, `intendedRegressions` or `sweep-known-failures.txt` to make a test pass, without Pierre's decision.

The same holds for the comparison with MJML: a normalization of the oracle (a difference it stops comparing) is Pierre's decision, with a test that the comparison still sees a real difference of the same kind.

### Where safe-mjml differs from MJML on purpose

These are Pierre's decisions. A case that shows one of them stays a known failure, with the rule as its reason, rather than being fixed:

- **No NaN.** Where MJML writes `NaN` (or `Infinity`) from `parseInt` or `parseFloat` of an empty or non-numeric value, such as `width:NaN%` for an empty column width, `width="NaN"` on a table or an icon, `height:NaNpx` for an empty hero height or `padding-bottom:NaN%` for a fluid hero without an image ratio, safe-mjml writes no NaN: it keeps the value as written, or leaves the property out. Examples: `column-width-empty` and `table-width-empty` in the matrix, `mj-hero-fluid-height-no-ratio`, the NaN lines of `sweep-known-failures.txt`.
- **A raw double quote in an attribute value is escaped.** Attribute values are kept as written, entities included, as MJML keeps them, and written without further escaping, except a raw `"`: it can only come from an attribute written in single quotes (`font-family='"A"'`), where MJML writes it raw and breaks the attribute; safe-mjml writes `&quot;`. `TestAttributeValuesNeverBreakTheirAttribute` sets an encoded quote on every allowed attribute and checks that none reaches the output raw.
- **A component directly under `<mjml>` gets a warning.** A known component written outside `mj-head` and `mj-body` (`<mjml><mj-section>…`) is dropped, as in MJML, but safe-mjml also reports it as a misplaced tag, which MJML does not: the template says why it is not rendered. `TestRootWarningsInDocumentOrder` checks it.
- **Negative widths as MJML.** Where MJML writes a negative width, set in the template (`mj-image width="-37px"`, `mj-body width="-601px"`) or computed from a padding wider than the room it is taken from (`width:-50px` for an image in a section whose padding is wider than the body), safe-mjml writes the same (Pierre's decision of 2026-11-18): clients ignore a negative width, so the content keeps its size, where 0 would hide it. Examples: `mj-section-padding-wider-than-body`, `mj-image-width-negative`, `mj-body-width-negative`. A content is still never wider than its parent.

## Attribute matrix (`matrix/`)

`matrix/` holds generated cases: each document sets one box attribute (width, padding, border, inner-*…) on one component, on the tag, empty, through `mj-class` or through `mj-attributes`, with non-round dimensions. Do not edit them by hand: add a target or a value to `tools/mjml-reference/matrix.mjs`, then run `node matrix.mjs` (`--check` reports documents out of date).

`TestMJMLMatrix` compares each one with its MJML reference. The documents that still differ are listed in `matrix/known-failures.txt` (`name: reason`); one that starts matching MJML fails the test until its line is removed. Their output is also snapshotted (`snapshots/matrix/`), so a known failure that changes shows up in the snapshot diff.

The `enum-*` documents set every value of every attribute MJML types as an enum (40 in MJML 5.4.1), the empty value, an invalid one (`bogus`), and the empty value over each lower layer that holds another value (`-empty-over-class`, `-attributes`, `-all`, `-parent`): a reader that tests the value MJML does not test only differs on those. They are built with `tools/mjml-reference/layers.mjs`, which the pre-release layer sweep (`SWEEP=1 tools/release-diff.sh`) shares.

## Author mistakes (`mistakes/`)

`tools/mjml-reference/mistakes.mjs` writes `mistakes/`: short realistic templates of `testdata/` with one author mistake each (a close tag forgotten, doubled or in another case, a comment with `--`, an attribute without a value, without quotes or twice, text before `<mjml>`, a CDATA section, an unknown tag...), with the reference MJML 5.4.1 renders for them; `rejected.json` lists the ones MJML cannot render minified. `TestAuthorMistakes` checks that safe-mjml renders each one as MJML does, or returns an error, never anything else (an email that silently lost the rest of its content). The documents rendered differently are listed in `mistakes-known-failures.txt` with the reason. The test logs the ones safe-mjml refuses: safe-mjml reads the template as XML outside ending tags, and is stricter than MJML there. Add a mistake to `mistakes.mjs` rather than a document by hand; `node mistakes.mjs --check` runs in the CI.

## MJML's official templates

`TestOfficialTemplates` compares MJML's 25 official email templates (github.com/mjmlio/email-templates, at the commit written in `tools/mjml-reference/official.mjs` and the CI) with their MJML 5.4.1 references. The repository has no license, so the templates are not copied here: the CI clones it, `node official.mjs SRC/templates DIR` writes them with their references, and `SAFE_MJML_OFFICIAL_DIR=DIR go test ./mjml -run TestOfficialTemplates` compares them. The ones that differ are listed in `official-known-failures.txt` with the reason.

## How the comparison with MJML reads styles

Inline styles are compared by the declarations that take effect, not in order: `color:red;font-size:12px` equals `font-size:12px;color:red`. But a longhand written before a shorthand that resets it is dropped (`testutils.EffectiveDeclarations`), so `padding:2px;padding-top:40px` (MJML) differs from `padding-top:40px;padding:2px`, where the padding-top is lost.

The parsed DOMs are compared too, but the HTML parser repairs missing tags (a `<tr>` or `</td>` left out): the raw sequence of tag names and conditional comment markers must also be the same as the reference's.

Text is compared as a client displays it: each run of whitespace in a text node is collapsed to one space on both sides (`comparableText`). The references are minified, and html-minifier (`collapseWhitespace`) collapses the whitespace of text, which MJML's render does not do; HTML displays both the same way. Where whitespace is displayed as written, in `<pre>`, `<textarea>`, `<script>`, `<style>` or under an inline `white-space:pre*`, it is compared as is; `mj-raw-white-space-pre` is a known failure for that reason, as the minifier collapses there too. When the DOMs differ in a way no other report names, the first differing text is reported (`firstTextDifference`), so that a difference is never dropped. `TestTextWhitespaceIsComparedAsDisplayed` checks both sides: whitespace alone passes, a missing word, or spaces in a `<pre>`, a `<textarea>` or under `white-space:pre`, fail.

Every document of `testdata/` must be in `expectedCases` (integration_test.go), in `knownFailures` (known_failures_test.go) or in `unlistedDocuments` with a reason, and every reference must have its document (`TestEveryDocumentIsListed`).

## Invariants and fuzzing

`TestRenderInvariants` checks, on every document of `testdata/` and `matrix/`, properties that hold whatever MJML writes:

- the text of the document is visible outside the Outlook-only conditional comments, both as the parser reads it and as the source writes it: every word of the text of `mj-body`, read from the source with regular expressions, is visible unless MJML's reference does not show it either (an invariant on the parsed tree alone cannot see what the parser loses);
- no pixel width in the body is not a finite number, or wider than both the body and every pixel width the source writes (a negative width is allowed, as MJML writes it);
- conditional comments open and close in turn, without nesting, and no VML tag (`<v:…>` or `</v:…>`) is outside an Outlook conditional comment (`[if mso | IE]`, `[if mso]`, `[if gte mso 9]`…);
- the document renders without error;
- two renders give the same HTML;
- rendering does not panic.

A document that breaks one on purpose, or as a known failure, goes in `invariantExceptions` for that invariant only, with its reason; its other invariants are still checked, and an exception that is no longer needed fails the test. `FuzzRender` mutates the documents and looks for panics and non-deterministic output: `go test ./mjml -run '^$' -fuzz FuzzRender -fuzztime 1m`. The release gate of the CI runs it for 2 minutes on every pull request, and keeps the input it failed on as an artifact (fuzz-findings): add that input to `testdata/fuzz/FuzzRender/` with the fix, where every `go test` replays it. A panic is returned by Render as an error whose cause is `ErrInternal`; the invariants still count it as a panic.

## Before a release

`tools/release-diff.sh` renders every document with the previous tag and with the working tree. It lists how many render differently (`VERBOSE=1` names them, `DIFF=1` shows the changes), then fails (`TestNoRegressionAgainstRelease`) when:

- a fragment of an MJML reference (a tag with its attributes and effective style, each of its attributes and style declarations on its own, a text with its entities decoded, a CSS rule, inside Outlook comments too) that the previous release wrote is no longer written, even in a document or a tag that already differed from MJML;
- the document, as a sequence of these fragments and of the conditional comment markers, is further from the reference than the previous release's (more tokens to delete or insert): this catches what is added, or moved into or out of an Outlook comment;
- a document that matched its MJML reference no longer does;
- the previous release failed on a document, or rendered nothing, that the working tree renders without matching MJML (check it by hand; one that matches MJML counts as fixed).

The layer sweep is also a conformance test, `TestLayerSweepParity`: every sweep document must match MJML 5.4.1 or be listed in `testdata/sweep-known-failures.txt` with its reason, and a listed document that matches fails until its line is removed. The CI runs it; locally, `(cd tools/mjml-reference && node layer-sweep.mjs /some/dir)` then `SAFE_MJML_SWEEP_DIR=/some/dir go test ./mjml -run TestLayerSweepParity` (skipped without the variable).

It then compares the render time of the two releases on the benchmarks they share (10, 100 and 1,000 sections, and the parse alone), run in turn six times each, and fails when the median of the working tree is more than `BENCH_TOLERANCE` (default 0.15) slower than the previous release's, unless `tools/accepted-slowdowns.txt` lists the benchmark with the reason (Pierre's decision). A review found every render 75% slower after a change no output test could see. `BENCH=0` skips it.

With `SWEEP=1` it also generates and checks the layer sweep, about 6,800 documents (needs `npm install` in `tools/mjml-reference`, a few minutes): run it before every release. Without an argument it compares with the latest tag before HEAD (with HEAD's tag when tracked files have uncommitted changes). The full report is printed on failure; `KEEP=1` keeps the previous release's output and prints the command that reruns the check on it. Each remaining difference must be explained in the release notes.

## Reference Implementation

The expected HTML outputs are rendered by the official MJML implementation (JavaScript), the canonical reference for MJML specification compliance. Earlier references came from MRML (Rust); since the move to MJML 4.18, and again for MJML 5.4.1, every reference is regenerated with `tools/mjml-reference/regenerate.mjs`, and `node regenerate.mjs --check` reports any reference that no longer matches MJML, or any input MJML refuses to render. `mjml` and `mj-raw-head` are kept as they are because MJML rejects them on purpose.

## Performance Testing

Some test cases are also used for benchmarking:
- `basic.mjml` - Simple performance baseline
- `complex-layout.mjml` - Complex template performance
- `austin-*.mjml` - Real-world template performance

Run benchmarks with: `./bench.sh`