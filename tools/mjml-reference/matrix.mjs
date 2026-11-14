// Generates the attribute matrix of mjml/testdata/matrix: small documents that
// set one box attribute (width, padding, border, inner-*...) on one component,
// in the forms MJML resolves differently (on the tag, empty, through mj-class,
// through mj-attributes), always with non-round dimensions (body widths of
// 601, 599 or 333px, columns of 33.33% or 200.9px, uneven paddings). The
// reference HTML of each document is rendered by the MJML version pinned in
// package.json, like regenerate.mjs does for mjml/testdata.
//
// Usage (from this directory, after `npm install`):
//   node matrix.mjs           # (re)write every document, its reference and warnings.json
//   node matrix.mjs --check   # report documents, references or warnings that are out of date
//
// warnings.json holds, per document that has some, the warnings of MJML's
// soft validation as safe-mjml writes them ("line N (tag): message", without
// the trailing space of mjml-core's default messages); TestMJMLMatrixWarnings
// compares safe-mjml's with them.
//
// Rounded dimensions hide truncation and rounding bugs, and attributes that are
// absent and empty behave differently in MJML: the fixtures written by hand
// covered neither, which let regressions through. The enum cases (enum-*) set
// every value of every enum attribute, the empty and an invalid one, and the
// empty one over each lower layer (layers.mjs). The inline cases (inline-*)
// give every component the class of an inline mj-style rule, on the element
// and through an mj-class. Add a target or a value here rather than
// hand-writing a matrix case.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { CONTEXTS, PASSING_PARENTS, components, enumValues, layeredDocument } from './layers.mjs';

const require = createRequire(import.meta.url);
const mjml2html = require('mjml');
const mjmlVersion = require('mjml/package.json').version;

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.resolve(here, '../../mjml/testdata/matrix');

const BODY_WIDTHS = [601, 599, 333];

// Values with odd numbers, so that a width computed or truncated at the wrong
// step shows up in the output.
const VALUES = {
  padding: ['7px 13px 9px 31px'],
  'padding-top': ['11px'],
  'padding-bottom': ['11px'],
  'padding-left': ['17px'],
  'padding-right': ['23px'],
  border: ['3px solid red', 'solid 3px red'],
  'border-top': ['2px solid green'],
  'border-bottom': ['2px solid green'],
  'border-left': ['5px dashed blue'],
  'border-right': ['5px dashed blue'],
  'border-radius': ['7px'],
  'inner-padding': ['9px 19px'],
  'inner-border': ['4px solid red'],
  'inner-border-top': ['4px solid red'],
  'inner-border-bottom': ['4px solid red'],
  'inner-border-left': ['4px solid red'],
  'inner-border-right': ['4px solid red'],
  'inner-border-radius': ['6px'],
  'inner-background-color': ['#ff0000'],
  'text-padding': ['3px 7px'],
  'icon-padding': ['5px'],
  'icon-height': ['33px'],
  'line-height': ['19px'],
  height: ['41px'],
  'max-height': ['57px'],
  gap: ['13px'],
  cellpadding: ['3'],
  'border-width': ['3px'],
  'border-style': ['dashed'],
  'border-color': ['red'],
};

// Box attributes of each target, from MJML 4.18's allowedAttributes, with the
// widths that make sense for it.
const TARGETS = {
  'mj-wrapper': { attrs: ['padding', 'padding-left', 'padding-right', 'padding-top', 'border', 'border-left', 'border-right', 'border-radius', 'gap'] },
  'mj-section': { attrs: ['padding', 'padding-left', 'padding-right', 'padding-top', 'padding-bottom', 'border', 'border-left', 'border-right', 'border-top', 'border-radius'] },
  'mj-group': { attrs: [], widths: ['67%', '401px'] },
  'mj-column': {
    attrs: ['padding', 'padding-left', 'padding-right', 'padding-top', 'border', 'border-left', 'border-right', 'border-radius',
      'inner-border', 'inner-border-left', 'inner-border-right', 'inner-border-radius', 'inner-background-color'],
    widths: ['33.33%', '200.9px'],
  },
  'mj-image': { attrs: ['padding', 'padding-left', 'padding-right', 'border', 'border-left', 'border-right', 'border-radius', 'height', 'max-height'], widths: ['333px', '77%'] },
  'mj-button': { attrs: ['padding', 'padding-left', 'padding-right', 'border', 'border-left', 'border-radius', 'inner-padding', 'height', 'line-height'], widths: ['201px', '67%'] },
  'mj-divider': { attrs: ['padding', 'padding-left', 'padding-right', 'border-width', 'border-style', 'border-color'], widths: ['77%', '211px'] },
  'mj-text': { attrs: ['padding', 'padding-left', 'padding-right', 'height', 'line-height'] },
  'mj-table': { attrs: ['padding', 'padding-left', 'border', 'cellpadding', 'line-height'], widths: ['81%', '303px'] },
  'mj-social': { attrs: ['padding', 'padding-left', 'inner-padding', 'icon-padding', 'icon-height', 'text-padding', 'border-radius', 'line-height'] },
  'mj-spacer': { attrs: ['padding', 'padding-left', 'border', 'border-left', 'height'] },
};

// Attributes whose resolution through mj-class and mj-attributes is also
// covered (the generic resolution is shared, so a subset is enough).
const RESOLUTION_ATTRS = new Set(['padding', 'padding-left', 'border', 'width', 'inner-padding', 'inner-border']);

// The leaves of the measured column: every component whose width depends on
// its container.
function leaves(target, attrs) {
  const a = (tag) => (tag === target ? attrs : '');
  return [
    `<mj-image src="https://example.com/image.png"${a('mj-image')} />`,
    `<mj-text${a('mj-text')}>Text in the measured column</mj-text>`,
    `<mj-button href="https://example.com"${a('mj-button')}>Button</mj-button>`,
    `<mj-divider${a('mj-divider')} />`,
    `<mj-table${a('mj-table')}><tr><td>Cell</td><td>Cell</td></tr></mj-table>`,
    `<mj-social${a('mj-social')}><mj-social-element name="facebook" href="https://example.com">Facebook</mj-social-element></mj-social>`,
    `<mj-spacer${a('mj-spacer')} />`,
  ].map((l) => `            ${l}`).join('\n');
}

function documentFor(target, attrs, headDefs, bodyWidth, nested) {
  const a = (tag) => (tag === target ? attrs : '');
  const head = headDefs ? `  <mj-head>\n    <mj-attributes>\n      ${headDefs}\n    </mj-attributes>\n  </mj-head>\n` : '';
  const column = `          <mj-column${a('mj-column')}>\n${leaves(target, attrs)}\n          </mj-column>\n          <mj-column>\n            <mj-text>Second column</mj-text>\n          </mj-column>`;
  const inner = nested
    ? `      <mj-section${a('mj-section')}>\n        <mj-group${a('mj-group')}>\n${column.replace(/^/gm, '  ')}\n        </mj-group>\n      </mj-section>`
    : `      <mj-section${a('mj-section')}>\n${column}\n      </mj-section>`;
  const body = nested ? `    <mj-wrapper${a('mj-wrapper')}>\n${inner.replace(/^/gm, '  ')}\n    </mj-wrapper>` : inner.replace(/^/gm, '  ');
  return `<mjml>\n${head}  <mj-body width="${bodyWidth}px">\n${body}\n  </mj-body>\n</mjml>\n`;
}

// cases returns every matrix document, keyed by name.
function cases() {
  const out = new Map();
  let n = 0;
  const add = (name, target, attrs, headDefs) => {
    const nested = target === 'mj-wrapper' || target === 'mj-group' || n % 2 === 1;
    out.set(name, documentFor(target, attrs, headDefs, BODY_WIDTHS[n % BODY_WIDTHS.length], nested));
    n++;
  };
  for (const bodyWidth of BODY_WIDTHS) {
    for (const nested of [false, true]) {
      out.set(`baseline-${bodyWidth}${nested ? '-nested' : ''}`, documentFor('', '', '', bodyWidth, nested));
    }
  }
  for (const [target, spec] of Object.entries(TARGETS)) {
    const entries = spec.attrs.flatMap((attr) => VALUES[attr].map((value, i) => [attr, value, i]));
    for (const [i, width] of (spec.widths || []).entries()) entries.push(['width', width, i]);
    for (const [attr, value, i] of entries) {
      const suffix = i > 0 ? `-${i + 1}` : '';
      const base = `${target.slice(3)}-${attr}${suffix}`;
      add(`${base}-tag`, target, ` ${attr}="${value}"`, '');
      if (i === 0) add(`${base}-empty`, target, ` ${attr}=""`, '');
      if (RESOLUTION_ATTRS.has(attr) && i === 0) {
        add(`${base}-class`, target, ' mj-class="m"', `<mj-class name="m" ${attr}="${value}" />`);
        add(`${base}-attributes`, target, '', `<${target} ${attr}="${value}" />`);
      }
    }
  }
  addEnumCases(out);
  addInlineCases(out);
  return out;
}

// INLINE_RULE is the inline mj-style rule of the inline-* cases: a color, a
// background color and a padding, the declarations juice also writes as
// attributes on table elements (align, valign, bgcolor, width), and the
// display:none !important authors hide elements with.
const INLINE_RULE = '.ix { color: #ff0000; background-color: #00ff00; padding: 7px; text-align: right; vertical-align: bottom; width: 61%; display: none !important; }';

// addInlineCases adds, for every component, a document whose element has
// the class of an inline mj-style rule, set on the element (inline-*-class)
// or through an mj-class (inline-*-mj-class): juice inlines the rule on the
// HTML elements that carry the class, which differ by component.
function addInlineCases(out) {
  const style = `<mj-style inline="inline">${INLINE_RULE}</mj-style>`;
  for (const tag of Object.keys(CONTEXTS)) {
    const base = `inline-${tag.slice(3)}`;
    for (const [name, layers] of [['class', { e: 'ix' }], ['mj-class', { c: 'ix' }]]) {
      const doc = layeredDocument(tag, 'css-class', layers).replace('<mj-head>', `<mj-head>${style}`);
      out.set(`${base}-${name}`, doc);
    }
  }
}

// addEnumCases adds, for every attribute MJML types as an enum, a document per
// value, one with the value empty and one with an invalid value, all on the
// element; then the empty value over each lower layer that holds a value other
// than the default (mj-class, the mj-attributes tag, mj-all, the parent).
// A reader that tests the value MJML does not test (mode == "vertical" where
// mjml-social tests === 'horizontal') agrees with MJML on every valid value
// and only differs on the others.
function addEnumCases(out) {
  for (const tag of Object.keys(CONTEXTS)) {
    const allowed = components[tag].allowedAttributes || {};
    const defaults = components[tag].defaultAttributes || {};
    for (const [attr, type] of Object.entries(allowed)) {
      const values = enumValues(type);
      if (!values) continue;
      const base = `enum-${tag.slice(3)}-${attr}`;
      for (const value of values.filter((v) => v !== '')) {
        out.set(`${base}-${value}`, layeredDocument(tag, attr, { e: value }));
      }
      out.set(`${base}-empty`, layeredDocument(tag, attr, { e: '' }));
      out.set(`${base}-invalid`, layeredDocument(tag, attr, { e: 'bogus' }));
      const other = values.find((v) => v !== '' && v !== defaults[attr]);
      const layers = { class: { c: other }, attributes: { t: other }, all: { a: other } };
      const path = CONTEXTS[tag].path;
      if (PASSING_PARENTS.has(path[path.length - 2])) layers.parent = { p: other };
      for (const [name, lower] of Object.entries(layers)) {
        out.set(`${base}-empty-over-${name}`, layeredDocument(tag, attr, { ...lower, e: '' }));
      }
    }
  }
}

const check = process.argv.includes('--check');
const docs = cases();
fs.mkdirSync(outDir, { recursive: true });
const stale = [];
const problems = [];
const warnings = {};
for (const [name, source] of docs) {
  const mjmlFile = path.join(outDir, `${name}.mjml`);
  const htmlFile = path.join(outDir, `${name}.html`);
  let html;
  try {
    const result = mjml2html(source, { minify: true, validationLevel: 'soft' });
    html = result.html;
    if (result.errors.length > 0) {
      warnings[name] = result.errors.map((e) => `line ${e.line} (${e.tagName}): ${e.message.trimEnd()}`);
    }
  } catch (err) {
    problems.push(`${name}: MJML failed: ${err.message}`);
    continue;
  }
  const current = (f) => (fs.existsSync(f) ? fs.readFileSync(f, 'utf8') : null);
  // Carousel and navbar IDs are random in each MJML run: a reference that
  // differs from the new rendering only by them is kept as it is.
  const withoutIds = (h) => h && h.replace(/[0-9a-f]{16}/g, 'ID');
  if (current(mjmlFile) === source && withoutIds(current(htmlFile)) === withoutIds(html)) html = current(htmlFile);
  if (current(mjmlFile) !== source || current(htmlFile) !== html) {
    stale.push(name);
    if (!check) {
      fs.writeFileSync(mjmlFile, source);
      fs.writeFileSync(htmlFile, html);
    }
  }
}
const warningsFile = path.join(outDir, 'warnings.json');
const sortedWarnings = Object.fromEntries(Object.keys(warnings).sort().map((k) => [k, warnings[k]]));
const warningsText = `${JSON.stringify(sortedWarnings, null, 2)}\n`;
if (!fs.existsSync(warningsFile) || fs.readFileSync(warningsFile, 'utf8') !== warningsText) {
  stale.push('warnings.json');
  if (!check) fs.writeFileSync(warningsFile, warningsText);
}
const extra = fs.readdirSync(outDir).filter((f) => f.endsWith('.mjml') && !docs.has(f.slice(0, -5)));
for (const f of extra) {
  stale.push(`${f.slice(0, -5)} (not generated any more)`);
  if (!check) {
    fs.rmSync(path.join(outDir, f));
    fs.rmSync(path.join(outDir, `${f.slice(0, -5)}.html`), { force: true });
  }
}
console.log(`MJML ${mjmlVersion}: ${docs.size} matrix documents, ${stale.length} ${check ? 'out of date' : 'written'}`);
for (const s of stale.slice(0, 20)) console.log(`  ${s}`);
for (const p of problems) console.log(`! ${p}`);
process.exitCode = check && (stale.length || problems.length) ? 1 : 0;
