# Differences from MJML 4.18

## Not supported

- `mj-include`. Assemble the template before rendering it.
- `mj-html-attributes`. It gives a `WarningUnsupportedTag` warning.
- Custom components (MJML's `registerComponent`).
- MJML's `minify`, `beautify` and `.mjmlconfig` options, and the CLI's `--watch`, `--config.*` and `--validate`. safe-mjml writes compact HTML, close to `mjml --minify`.
- An `mj-font` whose name contains `"` is not imported.
- In inline `mj-style`, some selectors are not applied (the rule is skipped):
  - cheerio's positional pseudo-classes: `:first`, `:last`, `:eq()`, `:nth()`, `:gt()`, `:lt()`, `:even`, `:odd`;
  - `:has()` with a sibling combinator: `:has(+ p)`, `:has(~ p)`;
  - rare forms: `p*`, `:has(:scope > b)`, `:nth-child(2n1)`, a line break in a quoted attribute value, an unescaped class starting with a digit (`.1a`), `:root()`, an escaped parenthesis or quote in `:contains()`.
- Lower case follows Go's rules, not JavaScript's, for a few letters such as `İ`.

## Stricter than MJML

These templates are refused with an error. MJML renders them.

- Malformed MJML outside content tags (`mj-text`, `mj-raw`, `mj-button`…): a missing or doubled close tag, text before `<mjml>`, an attribute without a value or without quotes. The error gives the line.
- Characters XML does not allow (control characters other than tab and line breaks, U+FFFE, U+FFFF) and invalid UTF-8.
- Content after `</mjml>` other than whitespace and comments.
- HTML larger than 16 MB.
- Inline `mj-style` that takes more than 250 million steps of work. A step is a rule tested on an element, a node or attribute read by a selector, 16 bytes read, 250 per declaration applied, 16 per byte of value, or a byte copied by a CSS variable. The costliest of MJML's official templates takes 62,468 steps.
- Inline `mj-style` nested more than 100 deep, and HTML nested more than 2,000 deep in a content tag when there is an inline `mj-style`.

## Different on purpose

- No `NaN`, `Infinity` or negative width. Where MJML writes `NaN` from an empty value, safe-mjml keeps the value as written or leaves the property out. Where MJML writes a negative width, safe-mjml writes 0. This covers 48 documents of the test corpora.
- A hexadecimal width (`width="0x10"`) on `mj-image`, `mj-button`, `mj-divider`, `mj-hero` or `mj-table` is read as 0. MJML reads 16 pixels.
- Template tags are kept intact by inline `mj-style`. MJML's juice protects `{{ }}` and `<% %>` but not Liquid's `{% %}`, and drops a tag placed alone in a `style` attribute. safe-mjml keeps them where they are.
- A `"` in an attribute value is escaped. MJML writes it raw.
- An `mj-navbar-link` without `href` in an `mj-navbar` with `base-url` links to the `base-url`. MJML appends `undefined`.
- A component directly under `<mjml>` (an `mj-section` outside `mj-body`) is dropped, as in MJML, but with a warning.
- In inline `mj-style`, a few rare cases:
  - a class starting with a hex escape (`.\32xl`) gets its normal specificity; in MJML it gets none;
  - `[title=\31 a]` matches `title="1a"`; MJML does not;
  - `$$` in a function other than `var()` stays `$$`; MJML writes `$`;
  - after an unclosed `var(`, no other `var()` of the value is resolved;
  - MJML writes an empty `style` attribute on the body's `div` and on mj-hero's cell, which `div[style]` matches; safe-mjml does not write it.

## Same as MJML, worth knowing

- `mj-carousel` writes CSS for every pair of images: its size grows with the square of the number of images. From about 960 images, the HTML goes over the 16 MB limit.
- Inline `mj-style` fails on a declaration without a name, as MJML does. Rich-text editors write `font-family: &quot;Helvetica Neue&quot;, Arial`, which juice splits at the `;` of `&quot;`. Write `'Helvetica Neue'` instead.
- Inline `mj-style` applies to the whole document with juice's selectors and CSS variables, and rewrites the document as juice does: attributes juice adds come after `style`, `<br />` becomes `<br>`, and CDATA sections become comments.
- A template tag must open and close in the same place. One opened in an attribute and closed in the text after it hides the markup in between from inline `mj-style`.
- A template tag inside the inline stylesheet (`.x { color: {{ brand }}; }`) breaks the CSS rule, in MJML too. Put such values in attributes.
