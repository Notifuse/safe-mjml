# Compatibility with MJML 5.4.1

safe-mjml implements every MJML 5.4.1 component. Its tests compare its output with what MJML 5.4.1 writes, not with what its authors expect. The MJML reference renderer is pinned in `tools/mjml-reference`: each reference is MJML's unminified render, compacted by html-minifier 4.0.0 with the options MJML 4 used for `minify: true` (`tools/mjml-reference/render.mjs`, [docs/mjml-5.md](mjml-5.md)).

## Test suites

All of them run on every pull request, except the guards, which run on main and before each release.

| Suite | Size | What it checks |
|---|---|---|
| Test cases (`mjml/testdata`) | 495 templates | The HTML matches MJML's: DOM, tag sequence, Outlook conditional comments, styles, stylesheets, doctype and visible text |
| Attribute matrix (`mjml/testdata/matrix`) | 613 documents | Every enum and size attribute, set, empty or invalid, directly and through `mj-class`, `mj-attributes` and `mj-all`; and MJML's validation warnings |
| Layer sweep (`tools/mjml-reference/layer-sweep.mjs`) | 6,987 documents | Each of the 385 attributes MJML allows on body components, at every level: element, `mj-class`, `mj-attributes`, `mj-all`, parent and grandparent |
| MJML's official templates ([mjmlio/email-templates](https://github.com/mjmlio/email-templates)) | 25 templates | Real emails written by MJML's team |
| MJML's own test suite (`tools/mjml-reference/mjml-suite.mjs`) | 69 documents | Every document the tests of MJML 5.4.1 render (`packages/mjml/test`), but those of the command line, `mj-include` and the JavaScript API |
| Author mistakes (`tools/mjml-reference/mistakes.mjs`) | 516 documents | One mistake each (a missing or doubled close tag, an unquoted attribute, text before `<mjml>`…): rendered as MJML does, or refused with its line |
| Liquid (`tools/mjml-reference/liquid.mjs`) | 90 documents | Liquid `{{ }}` and `{% %}` in attributes, content, around components and in the head, with and without inline `mj-style`: rendered as MJML does, and every tag MJML keeps is kept |
| Escaping | 1,706 documents | A quote in any attribute never breaks out of it |
| Invariants | every document above | The template's text is visible; no `NaN` or infinite width, and none wider than the body; conditional comments never nest; VML stays inside Outlook comments; two renders give the same HTML |
| Scaling | 52 families | Rendering time and memory grow linearly with the template, including hostile shapes |
| Fuzzing | 2 minutes | No panic, and the same output on every render |
| Guards (`tools/guard-mutations`) | 21 guards, on main | Each limit, budget, recovery and escape, removed in turn, makes a test fail |
| Snapshots | 1,108 documents | The HTML byte for byte |
| Release gate (`tools/release-diff.sh`) | 8,095 documents, 4 benchmarks | Compared with the previous release: no document gets further from MJML, and rendering is at most 15% slower |

CI also runs the tests with the race detector, on amd64 and arm64, and builds for 32-bit platforms. The rules for new tests are in [mjml/testdata/AGENTS.md](../mjml/testdata/AGENTS.md).

## Results

| Corpus | Documents | Identical to MJML 5.4.1 | The others |
|---|---|---|---|
| Test cases | 495 | 484 | 5 differ on purpose; 4 match MJML's output but not its compacted version; 2 are templates MJML refuses |
| Attribute matrix | 613 | 611 | 2 differ on purpose (no `NaN`) |
| Layer sweep | 6,987 | 6,944 | 43 differ on purpose (no `NaN`) |
| MJML's official templates | 25 | 25 | |
| MJML's own test suite | 69 | 65 of the 65 with a reference | html-minifier cannot compact MJML's render of 1, and removes the `htmlmin:ignore` markers MJML keeps in 3 |
| Author mistakes | 516 | rendered as MJML, or refused with their line | 5 keep a close tag that MJML writes and browsers ignore |
| Liquid | 90 | 82 of the 84 with a reference | 2 differ on purpose (an `mj-social-element` link to its network kept); the other 6 have no reference, and safe-mjml renders them with their Liquid: html-minifier cannot parse MJML's render of 4, and juice 11 throws on 2 whose inlined `style` holds Liquid and no declaration |

## Body components

Each component with every attribute MJML 5.4.1 allows on it but `name`, at every level:

| Component | Attributes | Documents | Identical to MJML |
|---|---|---|---|
| `mj-accordion` | 16 | 240 | all |
| `mj-accordion-element` | 11 | 297 | all |
| `mj-accordion-text` | 12 | 396 | all |
| `mj-accordion-title` | 10 | 330 | all |
| `mj-body` | 3 | 45 | all but 2 (no `NaN`) |
| `mj-button` | 32 | 480 | all |
| `mj-carousel` | 17 | 255 | all but 7 (no `NaN`) |
| `mj-carousel-image` | 10 | 270 | all |
| `mj-column` | 22 | 330 | all |
| `mj-divider` | 11 | 165 | all |
| `mj-group` | 4 | 60 | all but 1 (no `NaN`) |
| `mj-hero` | 20 | 300 | all but 7 (no `NaN`) |
| `mj-image` | 27 | 405 | all |
| `mj-navbar` | 22 | 330 | all |
| `mj-navbar-link` | 17 | 459 | all |
| `mj-section` | 23 | 345 | all |
| `mj-social` | 24 | 360 | all but 7 (no `NaN`) |
| `mj-social-element` | 30 | 810 | all but 12 (no `NaN`) |
| `mj-spacer` | 12 | 180 | all |
| `mj-table` | 19 | 285 | all but 7 (no `NaN`) |
| `mj-text` | 19 | 285 | all |
| `mj-wrapper` | 24 | 360 | all |

The head components (`mj-head`, `mj-title`, `mj-preview`, `mj-font`, `mj-style`, `mj-breakpoint`, `mj-attributes` with `mj-all` and `mj-class`) and `mj-raw` are covered by the test cases.

## Email clients

The HTML includes the Outlook conditional comments and VML backgrounds MJML writes, and targets the same clients: Outlook (desktop and Outlook.com), Gmail, Apple Mail, Yahoo Mail and Thunderbird.
