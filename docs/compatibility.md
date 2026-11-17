# Compatibility with MJML 4.18

safe-mjml implements every MJML 4.18 component. Its tests compare its output with what MJML 4.18 writes, not with what its authors expect. The MJML reference renderer is pinned in `tools/mjml-reference`.

## Test suites

All of them run on every pull request.

| Suite | Size | What it checks |
|---|---|---|
| Test cases (`mjml/testdata`) | 453 templates | The HTML matches MJML's: DOM, tag sequence, Outlook conditional comments, styles, stylesheets, doctype and visible text |
| Attribute matrix (`mjml/testdata/matrix`) | 610 documents | Every enum and size attribute, set, empty or invalid, directly and through `mj-class`, `mj-attributes` and `mj-all`; and MJML's validation warnings |
| Layer sweep (`tools/mjml-reference/layer-sweep.mjs`) | 6,915 documents | Each of the 381 attributes MJML allows on body components, at every level: element, `mj-class`, `mj-attributes`, `mj-all`, parent and grandparent |
| MJML's official templates ([mjmlio/email-templates](https://github.com/mjmlio/email-templates)) | 25 templates | Real emails written by MJML's team |
| Author mistakes (`tools/mjml-reference/mistakes.mjs`) | 516 documents | One mistake each (a missing or doubled close tag, an unquoted attribute, text before `<mjml>`…): rendered as MJML does, or refused with its line |
| Liquid (`tools/mjml-reference/liquid.mjs`) | 90 documents | Liquid `{{ }}` and `{% %}` in attributes, content, around components and in the head, with and without inline `mj-style`: rendered as MJML does, and every tag MJML keeps is kept |
| Escaping | 1,690 documents | A quote in any attribute never breaks out of it |
| Invariants | every document above | The template's text is visible; no negative or `NaN` width; conditional comments never nest; VML stays inside Outlook comments; two renders give the same HTML |
| Scaling | 42 families | Rendering time grows linearly with the template, including hostile shapes |
| Fuzzing | 2 minutes | No panic, and the same output on every render |
| Snapshots | 1,057 documents | The HTML byte for byte |
| Release gate (`tools/release-diff.sh`) | 7,968 documents, 4 benchmarks | Compared with the previous release: no document gets further from MJML, and rendering is at most 15% slower |

CI also runs the tests with the race detector, on amd64 and arm64, and builds for 32-bit platforms. The rules for new tests are in [mjml/testdata/AGENTS.md](../mjml/testdata/AGENTS.md).

## Results

| Corpus | Documents | Identical to MJML 4.18 | The others |
|---|---|---|---|
| Test cases | 453 | 442 | 5 differ on purpose; 4 match MJML's output but not its minified version; 2 are templates MJML refuses |
| Attribute matrix | 610 | 608 | 2 differ on purpose (no `NaN`) |
| Layer sweep | 6,915 | 6,872 | 43 differ on purpose (no `NaN`) |
| MJML's official templates | 25 | 25 | |
| Author mistakes | 516 | rendered as MJML, or refused with their line | 5 keep a close tag that MJML writes and browsers ignore |
| Liquid | 90 | 83 of the 86 MJML renders minified | 3 differ on purpose (Liquid kept in an inlined `style`; an `mj-social-element` link to its network kept); MJML cannot minify the other 4, which safe-mjml renders with their Liquid |

## Body components

Each component with every attribute MJML 4.18 allows on it, at every level:

| Component | Attributes | Documents | Identical to MJML |
|---|---|---|---|
| `mj-accordion` | 16 | 255 | all |
| `mj-accordion-element` | 11 | 314 | all |
| `mj-accordion-text` | 12 | 396 | all |
| `mj-accordion-title` | 10 | 330 | all |
| `mj-body` | 2 | 30 | all but 2 (no `NaN`) |
| `mj-button` | 33 | 536 | all |
| `mj-carousel` | 17 | 271 | all but 7 (no `NaN`) |
| `mj-carousel-image` | 10 | 270 | all |
| `mj-column` | 22 | 385 | all but 1 (no `NaN`) |
| `mj-divider` | 11 | 194 | all |
| `mj-group` | 4 | 80 | all but 1 (no `NaN`) |
| `mj-hero` | 21 | 323 | all but 7 (no `NaN`) |
| `mj-image` | 28 | 443 | all |
| `mj-navbar` | 22 | 346 | all |
| `mj-navbar-link` | 18 | 459 | all |
| `mj-section` | 22 | 386 | all |
| `mj-social` | 23 | 397 | all but 7 (no `NaN`) |
| `mj-social-element` | 30 | 809 | all but 12 (no `NaN`) |
| `mj-spacer` | 12 | 197 | all |
| `mj-table` | 19 | 339 | all but 8 (no `NaN`) |
| `mj-text` | 19 | 316 | all |
| `mj-wrapper` | 23 | 399 | all |

The head components (`mj-head`, `mj-title`, `mj-preview`, `mj-font`, `mj-style`, `mj-breakpoint`, `mj-attributes` with `mj-all` and `mj-class`) and `mj-raw` are covered by the test cases.

## Email clients

The HTML includes the Outlook conditional comments and VML backgrounds MJML writes, and targets the same clients: Outlook (desktop and Outlook.com), Gmail, Apple Mail, Yahoo Mail and Thunderbird.
