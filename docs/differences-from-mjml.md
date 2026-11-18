# Differences from MJML 5.4.1

## Not supported

- `mj-include`: the file it names is not read. Assemble the template before rendering it. What an `mj-include` holds is rendered in its place, as MJML 5 does, with a warning.
- `mj-html-attributes`. It gives a `WarningUnsupportedTag` warning.
- Custom components (MJML's `registerComponent`).
- MJML's `minify`, `beautify` and `.mjmlconfig` options, and the CLI's `--watch`, `--config.*` and `--validate`. safe-mjml writes compact HTML: MJML's render with its whitespace collapsed, as MJML 4 wrote it with `minify: true`.
- An `mj-font` whose name contains `"` is not imported.
- In inline `mj-style`, some selectors are not applied (the rule is skipped):
  - cheerio's positional pseudo-classes: `:first`, `:last`, `:eq()`, `:nth()`, `:gt()`, `:lt()`, `:even`, `:odd`;
  - rare forms: `p*`, `:has(:scope > b)`, `:nth-child(2n1)`, a line break in a quoted attribute value, an unescaped class starting with a digit (`.1a`), `:root()`;
  - css-select's parent combinator `<` (`p < div` styles the `div` that has a `p` child);
  - the namespace forms `*|*`, which MJML applies to every element of the document, the skeleton's included, and `[|title]`.
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

- No `NaN`, `Infinity` or negative width. Where MJML writes `NaN` from an empty value, safe-mjml keeps the value as written or leaves the property out. Where MJML computes a negative width from a padding wider than its container, safe-mjml writes 0; a negative width the template sets is written as MJML writes it. Where a column width overflows a number (1.7e308px in a 1px group) and MJML writes `Infinity`, safe-mjml writes 2147483647 (-2147483647 for `-Infinity`), and a gutter that would make it overflow is left out. Not yet covered, as in v1.0.1: an `mj-button` or `mj-divider` width beyond 1e308 writes `Infinity`, and `mj-divider width="%"` writes `NaN`, as MJML does (for the divider's `Infinity`, MJML writes a width in pixels). This covers 48 documents of the test corpora.
- A hexadecimal width (`width="0x10"`) on `mj-image`, `mj-button`, `mj-divider`, `mj-hero` or `mj-table` is read as 0. MJML reads 16 pixels.
- Template tags are kept intact by inline `mj-style`. MJML's juice protects `{{ }}` and `<% %>` but not Liquid's `{% %}`, and drops a tag placed alone in a `style` attribute. safe-mjml keeps them where they are.
- Inline `mj-style` renders an element whose `style` attribute keeps no declaration juice 11 reads (`style="{{ styles.paragraph }}"`, `style=";"`, a Liquid block juice does not protect, or `/* juice ignore next */` before its only declaration) when a rule reaches it. MJML 5.4.1 fails there, as juice 11 throws `Cannot read properties of undefined (reading '1')`. safe-mjml reads the attribute as MJML 4.18's juice 10 did: it writes the rule's declarations, keeps the template blocks, and keeps the declaration after the ignore comment.
- A `"` in an attribute value is escaped. MJML writes it raw.
- An `mj-social-element` whose `href` already points to its network (`name="x" href="https://x.com/notifuse"`, or a share URL) links to that `href`. MJML wraps every `href` in the network's share URL, which turns a profile link into a share link. A `-noshare` name (`x-noshare`) gives the same link in both.
- An `mj-navbar-link` without `href` in an `mj-navbar` with `base-url` links to the `base-url`. MJML appends `undefined`.
- A component directly under `<mjml>` (an `mj-section` outside `mj-body`) is dropped, as in MJML, but with a warning.
- In inline `mj-style`, a few rare cases:
  - a class starting with a hex escape (`.\32xl`) gets its normal specificity; in MJML it gets none;
  - `[title=\31 a]` matches `title="1a"`; MJML does not;
  - `$$` in a function other than `var()` stays `$$`; MJML writes `$`;
  - after an unclosed `var(`, no other `var()` of the value is resolved;
  - an upper-case hexadecimal escape followed by a space (`.\4A b`) is one class, `Jb`; MJML reads a class `J` and a descendant `b` (css-what reads the space after a lower-case escape only, `.\4a b`, as part of it);
  - MJML writes an empty `style` attribute on mj-hero's inner cell, which `td[style]` matches; safe-mjml does not write it.
- An empty MathML element, such as `<mi></mi>`, in a template with an inline `mj-style` is written `<mi/>`, as juice writes it. html-minifier, which minified MJML 4's output and compacts the references, then drops the slash and writes `<mi>`, which makes browsers nest the next element inside it.

## Same as MJML, worth knowing

- `mj-carousel` writes CSS for every pair of images: its size grows with the square of the number of images. From about 960 images, the HTML goes over the 16 MB limit.
- Inline `mj-style` fails on a declaration without a name, as MJML does. Rich-text editors write `font-family: &quot;Helvetica Neue&quot;, Arial`, which juice splits at the `;` of `&quot;`. Write `'Helvetica Neue'` instead.
- Inline `mj-style` applies to the whole document with juice's selectors and CSS variables, and rewrites the document as juice does: attributes juice adds come after `style`, `<br />` becomes `<br>`, and CDATA sections become comments.
- A template tag must open and close in the same place. One opened in an attribute and closed in the text after it hides the markup in between from inline `mj-style`.
- A template tag inside the inline stylesheet (`.x { color: {{ brand }}; }`) breaks the CSS rule, in MJML too. Put such values in attributes.
