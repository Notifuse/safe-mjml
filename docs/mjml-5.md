# MJML 5

safe-mjml 1.x targets MJML 4.18.0. This page summarizes a study of MJML 5.4.1, made shortly before safe-mjml 1.0.0: what MJML 5 changes in the HTML, what it would take for safe-mjml to follow it, and how the test references would be regenerated.

## Decision

- safe-mjml 1.x stays on MJML 4.18.0.
- If safe-mjml follows a later MJML, that MJML is the target of a new major version, and the references are regenerated once. safe-mjml does not offer two MJML versions as a render option: that would double the 8,500 or so reference documents and every known-failure list for good.
- The question reopens when MJML 6 is stable (6.0.0-alpha.1 was released on 2026-10-09), or if MJML 5's `gutter` is needed.

## What MJML 5 changes in the HTML

Measured on 8,487 documents: the test cases, the attribute matrix, the author mistakes and the layer sweep, as they were before 1.0.0. Both versions render 8,465 of them. Each document's two renders, unminified, were compared token by token with random IDs normalized, and every difference falls in one of these kinds:

| Change | Since | Documents |
|---|---|---|
| `mj-body` writes `<body>` itself: the body gets the css-class and a new `id` attribute, and keeps its background colour; the inner div gets `word-spacing:normal` and the background colour, and loses the class; the preview is written by `mj-body`, at the same place | 5.0.0 | 8,465 (all) |
| `mj-social` and `mj-social-element`: new `border` attribute, default `0`, inherited; the icon `<img>` gets `border:0` | 5.4.0 | 1,584 |
| `mj-accordion`: the Yahoo CSS selects `.mj-accordion-checkbox[type="checkbox"]` instead of `input.mj-accordion-checkbox` | 5.0.0 | 1,370 |
| `mj-column`: pixel widths rounded to whole pixels (`Math.round`), in the Outlook `<td>` width and the px class names | 5.x | 686 |
| `mj-hero`: children laid out in the container width minus the hero padding; `inner-padding` applies in every client, not only Outlook; the Outlook inner table width is read with `parseInt`; `container-background-color` is removed | 5.0.0 | 109 |
| Media queries grouped: one rule per distinct declaration list, with every class in its selector | 5.x | 2 |
| Inline `mj-style`: the content of a `<textarea>` is raw text again, so the rules do not apply inside it | 5.x | 1 |
| `mj-column`: mobile percentages cut to 6 decimals (`toFixed(6)`) | 5.x | 1 |
| `mj-section`: new `gutter` attribute, the space between columns, written as padding on each column with `mj-column-gutter-…` classes and media queries, mirrored in rtl | 5.4.0 | 0 (the corpora use 4.18's attributes) |
| `<!-- htmlmin:ignore -->` markers stripped | 5.x | 0 |

No component was added or removed.

### Validation

70 of the 8,487 documents warn differently:

- `border-radius` is a string on `mj-carousel` (and `tb-border-radius`), `mj-carousel-image`, `mj-column` (and `inner-border-radius`), `mj-image`, `mj-social` and `mj-social-element`: 67 warnings in 64 documents disappear.
- `container-background-color` on `mj-hero` is illegal: 12 new warnings in 10 documents.
- New attributes: `id` on `mj-body`, `border` on `mj-social` and `mj-social-element`, `gutter` on `mj-section`.

### Inline mj-style

- juice 11.1.1 (was 10.0.1), on htmlparser2 9.1.0 (was 8.0.2). css-select stays on 5.x.
- htmlparser2 9 reads `<textarea>` as raw text again. Its other changes are a refactor.
- juice 11 honours `/* juice ignore */`, `/* juice start ignore */ … /* juice end ignore */` and `/* juice ignore next */`. The juicer's Juice12 profile already does; its Juice9 and Juice10 profiles do not.
- juice 11's new `decodeStyleAttributes` option is off by default, and MJML does not set it.

### Node API (nothing for safe-mjml)

`mjml2html` is async; `mj-include` is ignored by default (safe-mjml has no `mj-include`); the `mjml-migrate` helper is removed; Node 20 or later is required.

## The minifier and the test references

MJML 5 minifies with htmlnano 3.3.1 and cssnano 8 ("lite" preset) instead of html-minifier:

- It rewrites CSS in `<style>` and in `style=""`: `Ubuntu,Helvetica` without spaces, `rgb(1,2,3)`, `none!important`, empty declarations dropped, no final `;`.
- It keeps comments by default, and single spaces between tags.
- cssnano throws on CSS it cannot parse. With `minify: true`, MJML 5 fails on 5 test cases: an entity in a style attribute, CDATA in an inline style, comment forms in an inline style, `!important` in a width.

So MJML 5's minified output cannot serve as references: the minifier alone changes 8,439 of 8,460 documents, and the comparison would need a CSS normalizer on both sides. Its unminified output cannot either, since the comparison relies on html-minifier dropping `style=""` and collapsing whitespace.

What works: render with MJML 5 unminified, then minify with html-minifier 4.0.0 and the options MJML 4.18 passes it:

```js
minify(html, { collapseWhitespace: true, minifyCSS: false, caseSensitive: true, removeEmptyAttributes: true })
```

Applied to MJML 4.18's unminified output, this gives the current references byte for byte (416 of 444 test cases; the 28 others differ only by random carousel IDs). Through references made this way, the differences safe-mjml would face are exactly the kinds above. Some show under other names, because the comparison pairs styled elements by index (the body change shifts them all) and counts `type=` in CSS: the accordion as "VML type attribute count", the rounded column widths as "MSO sequence", the hero as "MSO table width value".

## What it would take

| Change | Code | Size |
|---|---|---|
| `<body>` from `mj-body` | `mjml/render.go` (the skeleton, where `word-spacing:normal` is written today), `internal/components/body.go` | small; changes every document's bytes |
| Social `border` | `internal/components/social.go` | small |
| Accordion selectors | `internal/components/accordion.go` (one CSS string) | trivial |
| Column rounding and 6 decimals | `internal/components/column.go`, `width.go` | small; the rounding must be JavaScript's `Math.round` and `toFixed(6)`, not Go's |
| Section `gutter` | `section.go`, `column.go`, `group.go`, the media queries in `mjml/render.go` | medium: about 300 lines of MJML's JavaScript. julez-dev/mjmlgo's v2, which targets MJML 5.4.1, added about 650 lines of Go for gutter, hero and rounding |
| Hero | `internal/components/hero.go` | small to medium |
| Grouped media queries | `mjml/render.go` | small |
| Validation types | the attribute type tables, and the warnings in the tests | small |
| Inline `mj-style` | a Juice11 profile in `internal/juicer`: Juice10, plus the ignore comments, plus `textarea` in `rawTextEnds` of `internal/juicer/htmlparser2` for that profile only | small to medium; needs a parity run against juice 11.1.1 like the one made for juice 10 |
| `htmlmin:ignore` markers | where raw content and comments are written | trivial |

That is about a dozen commits, each with an MJML case, plus the tests:

- `tools/mjml-reference`: `regenerate.mjs`, `matrix.mjs`, `mistakes.mjs` and `layer-sweep.mjs` need the minify step above and `await`. html-minifier 4.0.0 becomes a direct dev dependency (today `mjml` 4.18 brings it).
- Every reference changes, through the body. The release gate needs a new baseline: a 2.x is compared with the previous 2.x release.
- The known lists need a review: `mistakes-known-failures.txt`, `sweep-known-failures.txt`, `official-known-failures.txt`, `intendedRegressions`, `invariantExceptions`, and the warnings in `expectedCases` (70 documents).
- New MJML cases for what the corpora do not cover: `gutter` (px, %, rtl, inside `mj-group`), the social `border`, the `mj-body` `id` and css-class, the hero padding, the grouped media queries, juice 11's ignore comments. They are written for safe-mjml: mjmlgo's parity fixtures are third-party files.

## For applications

- Every email changes by a few bytes that clients render the same: `word-spacing:normal` on the body div, `border:0` on social icons, the accordion CSS, Outlook column widths rounded to a whole pixel (under a pixel).
- One visible change: `mj-hero`'s `inner-padding` applies in every client, not only Outlook.
- With an inline `mj-style`: the markup inside a `<textarea>` is no longer styled, and `/* juice ignore */` comments are honoured.
- `gutter` is the one new feature.

## MJML 6

6.0.0-alpha.1 changes much more: 16px text and 150% line height by default, no Helvetica and Arial fallbacks, no `<tbody>`, the paragraph margin, a `support-outlook-classic` switch that drops the Outlook ghost tables, dark mode, responsive and ARIA attributes. Moving to MJML 5 now would mean moving again to MJML 6 soon after.

## Not covered

- MJML's official templates were not rendered with MJML 5; the kinds above cover them.
- No timing comparison.
- No parity run of juice 11 against the juicer: it needs the Juice11 profile first.
