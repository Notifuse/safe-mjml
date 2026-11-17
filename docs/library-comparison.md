# Go MJML libraries compared

Measured on 2026-11-17, with MJML 5.4.1 as the reference. The times were measured on 2026-11-17 too, and the binary sizes on 2026-11-14, with julez-dev/mjmlgo v1; the hostile templates at the end on 2026-11-17, with the versions of 2026-11-14. The two libraries that embed an older MJML or MRML were measured on 2026-11-12 against MJML 4.18, on the corpus of that day; their rows say so.

## Libraries

| Library | Version | How it renders | MJML version |
|---|---|---|---|
| [Notifuse/safe-mjml](https://github.com/Notifuse/safe-mjml) (this one) | v1.1.0 | native Go | 5.4.1 |
| [krozhkov/mgml](https://github.com/krozhkov/mgml) | v0.1.4 | native Go port of mjml | 4.18 |
| [preslavrachev/gomjml](https://github.com/preslavrachev/gomjml) | v0.15.0 | native Go | none stated |
| [julez-dev/mjmlgo](https://github.com/julez-dev/mjmlgo) | v2.0.0 | native Go | 5.4.1 (its v1 targets MJML 4.18) |
| [TcMits/mjml2html](https://github.com/TcMits/mjml2html) | v1.0.0 | mjml-browser in a JavaScript interpreter (goja) | 4.15.3 |
| [codedninja/mrmlgo](https://github.com/codedninja/mrmlgo) | v1.0.1 | [MRML](https://github.com/jdrouet/mrml) 5.0 (Rust) through cgo | MRML's own target |

**Left out:** [Boostport/mjml-go](https://github.com/Boostport/mjml-go) v0.16.0 (121 stars) and its fork [handsomefox/mjml-go](https://github.com/handsomefox/mjml-go). They run mjml-browser 4.15.3 compiled to WebAssembly (wazero), and they cannot be compared on this footing:
- they write MJML 4.15's HTML, so no document matches MJML 4.18 or 5.4.1;
- a render takes 2.7 s (6.7 s for the fork);
- they return an error and no HTML for any template MJML reports a validation error on, even with `validationLevel` soft, for instance `Attribute padding-top is illegal` on mj-accordion-element. That was 2,066 of the 7,855 documents on 2026-11-12.

## Documents

8,065 documents, each with the reference safe-mjml's tests use: the HTML MJML 5.4.1 writes for it (`validationLevel: 'soft'`), compacted by html-minifier as MJML 4 did for `minify: true` (`tools/mjml-reference/render.mjs`):
- the 465 test cases of `mjml/testdata` that have a reference;
- the 613 documents of the attribute matrix (`mjml/testdata/matrix`): every value of every attribute MJML types as an enum, and the size attributes;
- the 6,987 documents of the layer sweep (`tools/mjml-reference/layer-sweep.mjs`): every attribute MJML allows on each body component, with values and empty values, set on:
  - the element, an mj-class, the mj-attributes tag and mj-all;
  - the parent and the grandparent.

## Method

1. `tools/libcmp` renders every document with each library, with soft validation where the library has the option, and writes each output. A library that fails, panics or takes more than 30 s on a document counts a render failure.
2. `tools/libcmp/minify.cjs` passes every output through html-minifier with the options of the references. Each library is then compared on the same footing as the references.
3. `TestLibraryComparison` compares each output with MJML's through the comparison safe-mjml's own tests use: DOM, inline CSS, Outlook conditionals and the raw tag sequence. Before comparing, the random IDs of carousels and navbars are mapped onto MJML's, and an empty `class=""` inside an Outlook conditional is dropped. Neither changes what a client displays.
4. Time: the median of 15 renders of the Austin template, after 3 warm-up renders, one render at a time, on GitHub's Linux arm64 runner (`.github/workflows/bench.yml`). The figures of a laptop doing other work are 2 to 3 times higher.
5. Binary size: a program that reads a file and renders it with the library, built with Go 1.27.0 for macOS arm64 with `-trimpath -ldflags='-s -w'`, minus the same program without the render. The sizes published on 2026-11-12 came from another setup and are not comparable; the two libraries not measured again have none.

To run it again (julez-dev/mjmlgo needs Go 1.27, which `tools/libcmp` declares):

```bash
(cd tools/mjml-reference && npm ci && node layer-sweep.mjs /tmp/sweep)
cd tools/libcmp
{ find ../../mjml/testdata ../../mjml/testdata/matrix -maxdepth 1 -name '*.mjml'; find /tmp/sweep -name '*.mjml'; } \
  | sort | while read f; do [ -f "${f%.mjml}.html" ] && realpath "$f"; done > /tmp/libcmp/docs.txt
for lib in notifuse mgml preslav julez; do
  WORKERS=$([ $lib = mgml ] && echo 1 || echo 4) go run . $lib /tmp/libcmp/docs.txt /tmp/libcmp/out/$lib
  node minify.cjs /tmp/libcmp/out/$lib /tmp/libcmp/out-min/$lib
done
cd ../.. && SAFE_MJML_LIBCMP_DIR=/tmp/libcmp go test ./mjml -run TestLibraryComparison -v
```

The times come from the workflow "Benchmarks against MJML", started by hand.

## Results

| Library | Matches MJML 5.4.1 | Render failures | Austin template | Binary size added |
|---|---|---|---|---|
| Notifuse/safe-mjml | **8,013 (99.4%)** | 0 | 1.29 ms | 2.9 MB, of which 0.6 MB is the CSS inliner |
| julez-dev/mjmlgo v2 | 6 (0.1%); 2,488 (30.9%) with the whitespace inside Outlook comments ignored | 3,050 | 1.43 ms | 2.8 MB (v1) |
| krozhkov/mgml | 0; 6,569 of 7,969 (82.4%) matched MJML 4.18 on 2026-11-14 | 18 | 4.14 ms | 1.8 MB |
| preslavrachev/gomjml | 0; 1,457 of 7,969 (18.3%) matched MJML 4.18 on 2026-11-14 | 639 | 0.61 ms | 2.3 MB |
| TcMits/mjml2html (2026-11-12) | 0 of 7,855 (4,326 without the post-4.15 accessibility attributes) | 0 | 178 ms, Apple M5 | not measured again |
| codedninja/mrmlgo (2026-11-12) | 0 of 7,855 | 14 | 0.28 ms, Apple M5 | not measured again; needs cgo |

## Why each library differs or fails

- **Notifuse/safe-mjml.** Of the 52 documents that differ:
  - 50 do so by choice (see "Where safe-mjml differs from MJML on purpose" in `mjml/testdata/AGENTS.md`):
    - MJML writes a NaN width, height or padding for an empty value in most of them;
    - in the others it writes `Infinity`, a negative width, `undefined` after a navbar's base-url, or a share URL around a link to the network's own page;
  - 2 (mj-hero-fluid-height, mj-hero-inner-padding) match MJML in safe-mjml's own tests, which compare its output as written; here the second pass of the minifier changes them.
  - The 50 are listed with their reasons in `mjml/known_failures_test.go`, `mjml/testdata/matrix/known-failures.txt` and `mjml/testdata/sweep-known-failures.txt`.
- **julez-dev/mjmlgo.**
  - It was published on 2026-10-29. Its v1 targets MJML 4.18.0 and its v2 MJML 5.4.1; its own parity suite has 105 documents.
  - It writes the Outlook conditional comments of the head (`<noscript><xml><o:OfficeDocumentSettings>…`) and the head's CSS with another indentation than MJML's. That changes all but 6 of the 5,015 documents it renders. Ignoring the whitespace inside those comments, 2,488 documents match.
  - Among the 5,015, the comparison also finds differences of DOM in 1,926, of displayed text in 506 and of inline style in 265.
  - It fails on purpose ("fail-fast validation") on templates MJML renders with a warning. That is 3,050 documents, mostly from the layer sweep, which sets every attribute with an empty value too:
    - an empty value where it expects a unit (`attribute "padding" in <mj-section>: empty value is not a valid unit`, 1,289 documents) or a number (219);
    - an attribute it does not allow on that element (723);
    - a keyword outside its list, such as an alignment or a direction (484), or a colour it does not read (274);
    - other values it cannot parse, and malformed XML (61).
- **krozhkov/mgml.**
  - It ports MJML 4.18, and matched most of its documents (82.4% on 2026-11-14). Since MJML 5, mj-body writes the `<body>` element itself, with the background colour, and the div inside it changes too: no document matches MJML 5.4.1.
  - **It crashes the process when two renders run at the same time**: `fatal error: concurrent map writes`, in `core/types.InitializeType`, called from its validator. Go cannot recover from that error. It was measured one render at a time, so a server must serialise its calls.
  - 10 of its 18 render failures are panics: an index out of range, a slice out of bounds, a nil pointer, a division by zero (`mj-width-js-spaces`), a negative `strings.Repeat` (`mj-carousel-empty`). A server that does not recover them stops. Of the other 8, 5 are inline style sheets it cannot read (an at-rule, `:lang`, `:where`, a namespaced tag, an escape), and 3 have an unknown tag, which MJML renders with a warning.
- **preslavrachev/gomjml.**
  - It has no soft validation. Its render failures are templates it rejects for an attribute MJML accepts, for instance `Invalid attribute 'padding' for tag <mj-accordion-element>`.
  - It matched 18.3% of MJML 4.18's documents on 2026-11-14, and matches none of MJML 5.4.1's.
- **TcMits/mjml2html.**
  - It runs MJML itself, version 4.15.3. It was compared with MJML 4.18, which writes `aria-roledescription="email" role="article"` on the body's div, which 4.15 does not, so no document matched 4.18.
  - Without those two attributes, the remaining differences are MJML's own changes between 4.15 and 4.18.
- **codedninja/mrmlgo.**
  - MRML targets an earlier MJML: it writes no `lang` and `dir` on `<html>` or on the body's div, and writes its head styles differently, so no document matched MJML 4.18.
  - Its 14 render failures are templates its parser rejects (`unexpected token in root template`) and MJML renders.
  - It needs cgo and ships prebuilt static libraries for seven platforms.

## Hostile templates

Measured on 2026-11-17, on an Apple M5 laptop, with the versions of 2026-11-14 (safe-mjml v1.0.0, julez-dev/mjmlgo v1). Each check is a template written to cause harm, rendered by `go run . hostile <lib> <check>` in `tools/libcmp` (`tools/libcmp/hostile.go`); each runs in its own process, because one library crashes the process.

| Library | `"` in an attribute | `&quot;` in an attribute | 16 concurrent renders | 1 MB of inline styles |
|---|---|---|---|---|
| Notifuse/safe-mjml | escaped | kept escaped | ok | 0.15 s |
| krozhkov/mgml | **written raw** | kept escaped | **crashes the process** | **1 min 55 s** |
| preslavrachev/gomjml | escaped | kept escaped | ok | **15.5 s** |
| julez-dev/mjmlgo | **written raw** | **decoded and written raw** | ok | **30 s** |
| TcMits/mjml2html (MJML 4.15) | **written raw** | kept escaped | ok | not measured |
| Boostport/mjml-go (MJML 4.15) | **written raw** | kept escaped | ok | not measured |

- **A quote in an attribute.** The template sets `alt='x" onerror="alert(1)'` on an mj-image and a `href` with `" onclick="alert(2)` on an mj-button. Written raw, the quote closes the attribute and the rest becomes an `onerror` or `onclick` attribute of the HTML. MJML itself does this; safe-mjml escapes it (see `docs/differences-from-mjml.md`).
- **An escaped quote.** The same values with `&quot;`, as a template engine writes the data it inserts. julez-dev/mjmlgo decodes it and writes it raw, so data that was escaped correctly still adds an attribute.
- **Concurrent renders.** 16 goroutines render a small template 30 times each. krozhkov/mgml stops with `fatal error: concurrent map writes`, which Go cannot recover from.
- **Inline styles.** The family `inline-rules-and-elements` of `mjml/scaling_test.go`: n rules for one class in an inline `mj-style`, and n elements with that class, at 15, 61, 243 and 975 KB:

  | Library | 15 KB | 61 KB | 243 KB | 975 KB |
  |---|---|---|---|---|
  | Notifuse/safe-mjml | 3 ms | 8 ms | 36 ms | 146 ms |
  | krozhkov/mgml | 30 ms | 424 ms | 7.1 s | 1 min 55 s |
  | preslavrachev/gomjml | 3 ms | 38 ms | 634 ms | 15.5 s |
  | julez-dev/mjmlgo | 11 ms | 131 ms | 1.8 s | 30 s |

  Four times the template takes four times as long in safe-mjml, and 16 to 24 times as long in the three others: their time grows with the square of the template.
