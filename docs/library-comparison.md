# Go MJML libraries compared

Measured on 2026-11-18, at 43de726, with MJML 5.4.1 as the reference and the versions below. Every row, the times, the binary sizes and the hostile templates were measured that day, with the same versions.

## Libraries

| Library | Version | How it renders | MJML version |
|---|---|---|---|
| [Notifuse/safe-mjml](https://github.com/Notifuse/safe-mjml) (this one) | v1.1.0 | native Go | 5.4.1 |
| [julez-dev/mjmlgo](https://github.com/julez-dev/mjmlgo) | v2.0.0 | native Go | 5.4.1 (its v1 targets MJML 4.18) |
| [krozhkov/mgml](https://github.com/krozhkov/mgml) | v0.1.4 | native Go port of mjml | 4.18 |
| [preslavrachev/gomjml](https://github.com/preslavrachev/gomjml) | v0.16.0 | native Go | none stated |
| [TcMits/mjml2html](https://github.com/TcMits/mjml2html) | v1.0.0 | mjml-browser in a JavaScript interpreter (goja) | 4.15.3 |
| [codedninja/mrmlgo](https://github.com/codedninja/mrmlgo) | v1.0.1 | [MRML](https://github.com/jdrouet/mrml) 5.0 (Rust) through cgo | MRML's own target |

**Left out:** [Boostport/mjml-go](https://github.com/Boostport/mjml-go) v0.16.0 (121 stars) and its fork [handsomefox/mjml-go](https://github.com/handsomefox/mjml-go) v0.16.0. They run mjml-browser 4.15.3 compiled to WebAssembly (wazero), and they cannot be compared on this footing:
- they write MJML 4.15's HTML, so no document matches MJML 5.4.1;
- a render of the Austin template takes 3.1 s, on the machine of the hostile templates below;
- they return an error and no HTML for any template MJML reports a validation error on, even with `validationLevel` soft, for instance `Attribute padding-top is illegal` on mj-accordion-element. That was 2,066 of the 7,855 documents on 2026-11-12, with the same versions.

## Documents

8,093 documents, each with the reference safe-mjml's tests use: the HTML MJML 5.4.1 writes for it (`validationLevel: 'soft'`), compacted by html-minifier as MJML 4 did for `minify: true` (`tools/mjml-reference/render.mjs`):
- the 493 test cases of `mjml/testdata` that have a reference;
- the 613 documents of the attribute matrix (`mjml/testdata/matrix`): every value of every attribute MJML types as an enum, and the size attributes;
- the 6,987 documents of the layer sweep (`tools/mjml-reference/layer-sweep.mjs`): every attribute MJML allows on each body component, with values and empty values, set on:
  - the element, an mj-class, the mj-attributes tag and mj-all;
  - the parent and the grandparent.

## Method

1. `tools/libcmp` renders every document with each library, with soft validation where the library has the option, and writes each output. A library that fails, panics or takes more than 30 s on a document counts a render failure.
2. `tools/libcmp/minify.cjs` passes every output through html-minifier with the options of the references. Each library is then compared on the same footing as the references.
3. `TestLibraryComparison` compares each output with MJML's through the comparison safe-mjml's own tests use: DOM, inline CSS, Outlook conditionals and the raw tag sequence. Before comparing, the random IDs of carousels and navbars are mapped onto MJML's, and an empty `class=""` inside an Outlook conditional is dropped. Neither changes what a client displays.
4. Time: the median of 15 renders of the Austin template, after 3 warm-up renders, one render at a time, on GitHub's Linux arm64 runner (`.github/workflows/bench.yml`). The figures of a laptop doing other work are 2 to 3 times higher.
5. Binary size: a program that reads a file and renders it with the library, built with Go 1.27.0 with `-trimpath -ldflags='-s -w'`, minus the same program without the render (`tools/libcmp/sizes.sh`), on the same runner, for linux/arm64.

To run it again (julez-dev/mjmlgo needs Go 1.27, which `tools/libcmp` declares):

```bash
(cd tools/mjml-reference && npm ci && node layer-sweep.mjs /tmp/sweep)
cd tools/libcmp
{ find ../../mjml/testdata ../../mjml/testdata/matrix -maxdepth 1 -name '*.mjml'; find /tmp/sweep -name '*.mjml'; } \
  | sort | while read f; do [ -f "${f%.mjml}.html" ] && realpath "$f"; done > /tmp/libcmp/docs.txt
for lib in notifuse julez mgml preslav tcmits mrml; do
  WORKERS=$([ $lib = mgml ] && echo 1 || echo 4) go run . $lib /tmp/libcmp/docs.txt /tmp/libcmp/out/$lib
  node minify.cjs /tmp/libcmp/out/$lib /tmp/libcmp/out-min/$lib
done
cd ../.. && SAFE_MJML_LIBCMP_DIR=/tmp/libcmp go test ./mjml -run TestLibraryComparison -v
```

The times and the binary sizes come from the workflow "Benchmarks against MJML", started by hand.

## Results

| Library | Matches MJML 5.4.1 | Render failures | Austin template | Binary size added |
|---|---|---|---|---|
| Notifuse/safe-mjml | **8,036 (99.3%)** | 0 | 1.29 ms | 3.1 MB, of which 1.1 MB is the CSS inliner |
| julez-dev/mjmlgo | 6 (0.1%); 2,407 (29.7%) with the whitespace inside Outlook comments ignored | 3,068 | 1.54 ms | 2.8 MB |
| krozhkov/mgml | 0; it targets MJML 4.18 | 21 | 4.14 ms | 1.9 MB |
| preslavrachev/gomjml | 0 | 648 | 0.60 ms | 2.2 MB |
| TcMits/mjml2html | 0; it runs MJML 4.15 | 0 | 249 ms | 8.0 MB |
| codedninja/mrmlgo | 0; it runs MRML | 60 | 0.31 ms | 2.0 MB, and needs cgo |

## Why each library differs or fails

- **Notifuse/safe-mjml.** Of the 57 documents that differ:
  - 50 do so by choice (see "Where safe-mjml differs from MJML on purpose" in `mjml/testdata/AGENTS.md`):
    - MJML writes a NaN width, height or padding for an empty value in most of them;
    - in the others it writes `Infinity`, `NaNpx` for a divider width of `%`, a share URL around a link to the network's own page, or `undefined` after a navbar's base-url.
    - They are listed with their reasons in `mjml/known_failures_test.go`, `mjml/testdata/matrix/known-failures.txt` and `mjml/testdata/sweep-known-failures.txt`.
  - 7 match MJML in safe-mjml's own tests, which compare its output as written: mj-body-width-negative, mj-hero-fluid-height, mj-hero-inner-padding and four of the mj-html-attributes cases. Here the outputs go through the minifier a second time, and that second pass changes MJML's reference itself; minified again, the reference matches safe-mjml's output.
- **julez-dev/mjmlgo.**
  - It was published on 2026-10-29. Its v1 targets MJML 4.18.0 and its v2 MJML 5.4.1; its own parity suite has 105 documents.
  - It writes the Outlook conditional comments of the head (`<noscript><xml><o:OfficeDocumentSettings>…`) and the head's CSS with another indentation than MJML's. That changes all but 6 of the 5,025 documents it renders. Ignoring the whitespace inside those comments, 2,407 documents match.
  - Among the 5,025, the comparison also finds differences of DOM in 1,934, of displayed text in 511, of inline style in 267, of VML in 63 and of background in 31.
  - It fails on purpose ("fail-fast validation") on templates MJML renders with a warning. That is 3,068 documents, mostly from the layer sweep, which sets every attribute with an empty value too:
    - an empty value where it expects a unit (`attribute "padding" in <mj-section>: empty value is not a valid unit`, 1,289 documents) or a number (`strconv.Atoi: parsing "": invalid syntax`, 220);
    - an attribute it does not allow on that element (723), such as `padding` on mj-accordion-element;
    - a keyword outside its list, such as an alignment or a direction (483), or a colour it does not read (274);
    - other values it cannot parse, malformed XML MJML repairs, and `mj-include` (79).
- **krozhkov/mgml.**
  - It ports MJML 4.18. Since MJML 5, mj-body writes the `<body>` element itself, with the background colour, and the div inside it changes too: no document matches MJML 5.4.1. It matched 6,569 of 7,969 documents (82.4%) of MJML 4.18 on 2026-11-14.
  - **It crashes the process when two renders run at the same time**: `fatal error: concurrent map writes` or `concurrent map read and map write`, in `core/types.InitializeType`, called from its validator, in 20 of 21 runs of the check below. Go cannot recover from that error. It was measured one render at a time, so a server must serialise its calls.
  - 11 of its 21 render failures are panics: a slice out of bounds (unclosed comments, CDATA and ending tags, inline style comments), a nil pointer (`mj-social-element-unknown`), a division by zero (`mj-width-js-spaces`, `mj-container-width-exponent`), an index out of range, a negative `strings.Repeat` (`mj-carousel-empty`). A server that does not recover them stops. Of the other 10, 5 are inline style sheets it cannot read (an at-rule, `:lang`, `:where`, a namespaced tag, an escape), and 5 have an unknown tag, which MJML renders with a warning.
- **preslavrachev/gomjml.**
  - It has no soft validation. 612 of its 648 render failures are templates it rejects for an attribute MJML accepts, for instance `Invalid attribute 'padding' for tag <mj-accordion-element>` or `Invalid attribute 'gutter' for tag <mj-section>`; the 36 others are malformed XML MJML repairs and components it requires that MJML does not (a carousel without an image, an image without `src`).
  - It writes MJML 4's body, and matches none of MJML 5.4.1's documents. v0.15.0 matched 1,457 of MJML 4.18's 7,969 documents (18.3%) on 2026-11-14.
- **TcMits/mjml2html.**
  - It runs MJML itself, version 4.15.3. MJML 5 writes the `<body>` element itself, and 4.15 does not write `aria-roledescription="email" role="article"` on the body's div: no document matches, with or without those two attributes.
  - The JavaScript interpreter makes each render about 190 times slower than safe-mjml.
- **codedninja/mrmlgo.**
  - MRML targets an earlier MJML: it writes no `lang` and `dir` on `<html>` or on the body's div, and writes its head styles differently, so no document matches.
  - Its 60 render failures are all test cases its parser rejects and MJML renders: `mj-html-attributes` (13), comments, `mj-raw` inside components, unclosed ending tags, CDATA, a doctype or an XML declaration before `<mjml>`.
  - It does not inline an inline `mj-style`: the rule stays in a `<style>` element, so a client that ignores `<style>` does not apply it.
  - It needs cgo and ships prebuilt static libraries for seven platforms.

## Hostile templates

Measured on 2026-11-18, with the versions above, on a Linux amd64 machine with 4 CPUs (Intel Xeon at 2.3 GHz). Each check is a template written to cause harm, rendered by `go run . hostile <lib> <check>` in `tools/libcmp` (`tools/libcmp/hostile.go`); each runs in its own process, because one library crashes the process.

| Library | `"` in an attribute | `&quot;` in an attribute | 16 concurrent renders | 1 MB of inline styles |
|---|---|---|---|---|
| Notifuse/safe-mjml | escaped | kept escaped | ok | 0.35 s |
| julez-dev/mjmlgo | **written raw** | **decoded and written raw** | ok | **1 min 19 s** |
| krozhkov/mgml | **written raw** | kept escaped | **crashes the process** | **5 min** |
| preslavrachev/gomjml | escaped | kept escaped | ok | refused from 61 KB |
| TcMits/mjml2html (MJML 4.15) | **written raw** | kept escaped | ok | **over 15 min** |
| codedninja/mrmlgo | escaped | kept escaped | ok | does not inline |
| Boostport/mjml-go (MJML 4.15) | **written raw** | kept escaped | ok | not measured |

- **A quote in an attribute.** The template sets `alt='x" onerror="alert(1)'` on an mj-image and a `href` with `" onclick="alert(2)` on an mj-button. Written raw, the quote closes the attribute and the rest becomes an `onerror` or `onclick` attribute of the HTML. MJML itself does this; safe-mjml escapes it (see `docs/differences-from-mjml.md`).
- **An escaped quote.** The same values with `&quot;`, as a template engine writes the data it inserts. julez-dev/mjmlgo decodes it and writes it raw, so data that was escaped correctly still adds an attribute.
- **Concurrent renders.** 16 goroutines render a small template 30 times each. krozhkov/mgml stops with `fatal error: concurrent map writes`, which Go cannot recover from, in 20 of 21 runs.
- **Inline styles.** The family `inline-rules-and-elements` of `mjml/scaling_test.go`: n rules for one class in an inline `mj-style`, and n elements with that class, at 15, 61, 243 and 975 KB. codedninja/mrmlgo leaves the rules in a `<style>` element, without inlining them:

  | Library | 15 KB | 61 KB | 243 KB | 975 KB |
  |---|---|---|---|---|
  | Notifuse/safe-mjml | 5 ms | 17 ms | 76 ms | 348 ms |
  | julez-dev/mjmlgo | 19 ms | 295 ms | 4.2 s | 1 min 19 s |
  | krozhkov/mgml | 62 ms | 963 ms | 15.4 s | 5 min 1 s |
  | TcMits/mjml2html | 4.0 s | 1 min 4 s | over 15 min | not run |

  Four times the template takes about four times as long in safe-mjml, and 14 to 20 times as long in the three others: their time grows with the square of the template.

  preslavrachev/gomjml writes every declaration of every rule on each element, so its HTML grows with the square of the template too: at 15 KB it renders in 5 ms and writes `color: red` 300 times on each element, where MJML and safe-mjml write it once. v0.16.0 refuses the three larger templates with an error, `inline mj-style rules apply too many declarations to elements`; v0.15.0 rendered them, in 15.5 s at 975 KB on 2026-11-17.
