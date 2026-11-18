// Generates mjml/testdata/mistakes: realistic templates of mjml/testdata with
// one author mistake each (a close tag forgotten, doubled or without its
// ">", another case, a
// comment with "--", an attribute without a value or without quotes, text
// before <mjml>...), and the reference HTML MJML renders for them, minified
// as regenerate.mjs does.
//
// The test documents were all valid: a review then found that an ending tag
// left open before its parent's close dropped the rest of the email without
// an error, a mistake no document had. TestAuthorMistakes checks that safe-mjml
// renders each of these documents as MJML does, or returns an error; never
// anything else.
//
// Usage (from this directory, after `npm install`):
//   node mistakes.mjs           # (re)write every document, its reference and rejected.json
//   node mistakes.mjs --check   # report documents or references that are out of date
//
// rejected.json lists the documents render.mjs cannot render (MJML
// throws, or html-minifier refuses its output), with the message: there is
// no reference to compare with, and the test checks that safe-mjml does not
// panic on them, and refuses those MJML throws "Malformed MJML" for.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { renderReference, mjmlVersion } from './render.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const testdata = path.resolve(here, '../../mjml/testdata');
const outDir = path.join(testdata, 'mistakes');

// Short, realistic templates covering every component.
const BASES = [
  'basic',
  'austin-header-section',
  'austin-text-with-links',
  'austin-buttons',
  'austin-social-media',
  'austin-two-column-text',
  'austin-hero-images',
  'austin-wrapper-basic',
  'mj-accordion-icon',
  'mj-style-inline-selectors',
  'mj-class-tag-defaults',
  'notifuse-wrapper-bgcolor',
  'mj-navbar',
  'mj-carousel',
];

const ENDING_TAGS = ['mj-text', 'mj-button', 'mj-raw', 'mj-table', 'mj-navbar-link', 'mj-social-element', 'mj-accordion-title', 'mj-accordion-text'];
const CONTAINERS = ['mj-wrapper', 'mj-section', 'mj-column', 'mj-group', 'mj-hero', 'mj-navbar', 'mj-social', 'mj-accordion', 'mj-accordion-element', 'mj-carousel'];

// Each mistake returns the mutated source, or null when the base has nothing
// it applies to.
const replaceFirst = (s, from, to) => (s.includes(from) ? s.replace(from, to) : null);
// The end of the first open tag <name ...> (index after its >), or -1.
function afterOpenTag(s, name) {
  const m = new RegExp(`<${name}(?=[\\s/>])[^>]*>`).exec(s);
  return m && !m[0].endsWith('/>') ? m.index + m[0].length : -1;
}
const insertAfterOpen = (s, name, text) => {
  const i = afterOpenTag(s, name);
  return i < 0 ? null : s.slice(0, i) + text + s.slice(i);
};
const insertBefore = (s, name, text) => {
  const i = s.search(new RegExp(`<${name}(?=[\\s/>])`));
  return i < 0 ? null : s.slice(0, i) + text + s.slice(i);
};
// Adds attrs to the first <name ...>, or returns null when that tag already
// has one of them (unless duplicate is set): a mistake must test what its
// name says, not a duplicated attribute.
const addToOpenTag = (s, name, attrs, duplicate = false) => {
  const m = new RegExp(`<${name}(?=[\\s/>])[^>]*>`).exec(s);
  if (!m) return null;
  const names = [...attrs.matchAll(/([a-z-]+)(?==|\s|$)/g)].map((a) => a[1]);
  if (!duplicate && names.some((n) => new RegExp(`\\s${n}(?=[\\s=/>])`).test(m[0]))) return null;
  const at = m.index + name.length + 1;
  return s.slice(0, at) + ' ' + attrs + s.slice(at);
};

const MISTAKES = {};
for (const tag of ENDING_TAGS) {
  MISTAKES[`no-close-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, '');
  MISTAKES[`close-without-gt-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, `</${tag}\n`);
  MISTAKES[`close-upper-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, `</${tag.toUpperCase()}>`);
  MISTAKES[`close-space-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, `</ ${tag}>`);
  // "</" before anything but a letter opens a comment for htmlparser2, up
  // to the next ">": the close is lost in it.
  MISTAKES[`close-doubled-slash-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, `</</${tag}>`);
}
for (const tag of CONTAINERS) {
  MISTAKES[`no-close-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, '');
  MISTAKES[`close-without-gt-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, `</${tag}\n`);
  MISTAKES[`close-upper-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, `</${tag.toUpperCase()}>`);
  MISTAKES[`close-space-${tag}`] = (s) => replaceFirst(s, `</${tag}>`, `</ ${tag}>`);
}
Object.assign(MISTAKES, {
  'open-upper-mj-text': (s) => (s.includes('<mj-text') ? s.replace(/<mj-text(?=[\s>])/, '<MJ-TEXT').replace('</mj-text>', '</MJ-TEXT>') : null),
  'stray-close-p-in-column': (s) => insertAfterOpen(s, 'mj-column', '</p>'),
  'stray-close-mj-text-in-section': (s) => insertAfterOpen(s, 'mj-section', '</mj-text>'),
  'stray-close-mj-column-in-body': (s) => insertAfterOpen(s, 'mj-body', '</mj-column>'),
  'comment-with-dashes': (s) => insertBefore(s, 'mj-section', '<!-- ---------- BLOCK ---------- -->\n'),
  // The parent's close without its ">" inside an mj-text, before the
  // mj-text's own close: the shape that made safe-mjml panic (found by the
  // fuzzer, not by a single mistake above).
  'parent-close-without-gt-in-mj-text': (s) => replaceFirst(s, '</mj-text>', '\n</mj-column\n</mj-text>'),
  'comment-with-double-dash': (s) => insertBefore(s, 'mj-section', '<!-- old -- block -->\n'),
    'bare-attribute': (s) => addToOpenTag(s, 'mj-section', 'full-width'),
  'unquoted-attribute': (s) => addToOpenTag(s, 'mj-section', 'padding=11px'),
  'single-quoted-attribute': (s) => addToOpenTag(s, 'mj-text', "color='#123456'"),
  'duplicate-attribute': (s) => addToOpenTag(s, 'mj-text', 'color="#ff0000" color="#0000ff"', true),
  'text-before-root': (s) => `Hello\n${s}`,
  'text-in-section': (s) => insertAfterOpen(s, 'mj-section', 'Stray text'),
  'text-in-column': (s) => insertAfterOpen(s, 'mj-column', 'Stray text'),
  'html-in-column': (s) => insertAfterOpen(s, 'mj-column', '<p>Paragraph</p>'),
  'unknown-tag-in-column': (s) => insertAfterOpen(s, 'mj-column', '<mj-foo>Unknown</mj-foo>'),
  'mj-text-in-section': (s) => insertAfterOpen(s, 'mj-section', '<mj-text>Misplaced</mj-text>'),
  'self-closed-mj-text': (s) => insertAfterOpen(s, 'mj-column', '<mj-text />'),
  'nested-mj-text': (s) => insertAfterOpen(s, 'mj-column', '<mj-text>Outer <mj-text>inner</mj-text> after</mj-text>'),
  'amp-and-lt-in-text': (s) => insertAfterOpen(s, 'mj-text', ' Fish & chips < 5 euros '),
  'amp-in-attribute': (s) => addToOpenTag(s, 'mj-button', 'title="Fish & chips"'),
  'cdata-in-text': (s) => insertAfterOpen(s, 'mj-text', '<![CDATA[ raw <b>text</b> ]]>'),
  'close-with-attributes': (s) => replaceFirst(s, '</mj-text>', '</mj-text foo="bar">'),
  // What follows </mjml>: mjml-parser-xml takes the last element of the
  // document as its root, so MJML throws "Malformed MJML" for an element
  // there (a section pasted below the document, or the rest of a document
  // whose </mjml> is in an mj-text), and renders the second of two
  // documents. safe-mjml refuses all three (Pierre's rule).
  'element-after-root': (s) => `${s.trimEnd()}\n<mj-section><mj-column><mj-text>Pasted below the document</mj-text></mj-column></mj-section>\n`,
  'second-root': (s) => `${s.trimEnd()}\n<mjml><mj-body><mj-section><mj-column><mj-text>A second document</mj-text></mj-column></mj-section></mj-body></mjml>\n`,
  'root-close-in-mj-text': (s) => replaceFirst(s, '</mj-text>', '</mjml>'),
    });

// Left out, as no minified reference can be compared with safe-mjml: MJML
// passes the content of ending tags as written, and html-minifier repairs
// it in the reference (a stray </div> or an unclosed <b> in an mj-text, the
// rest of a document after an unended comment in an mj-raw), where safe-mjml
// writes what MJML renders (checked by hand on MJML's unminified render; the
// unended comment is mj-raw-unclosed-comment and mj-ending-tag-unclosed).

function documents() {
  const out = new Map();
  for (const base of BASES) {
    const source = fs.readFileSync(path.join(testdata, `${base}.mjml`), 'utf8');
    for (const [name, mistake] of Object.entries(MISTAKES)) {
      const mutated = mistake(source);
      if (mutated !== null && mutated !== source) out.set(`${base}--${name}`, mutated);
    }
  }
  return out;
}

const check = process.argv.includes('--check');
const docs = documents();
fs.mkdirSync(outDir, { recursive: true });
const stale = [];
const rejected = {};
const current = (f) => (fs.existsSync(f) ? fs.readFileSync(f, 'utf8') : null);
// Carousel and navbar IDs are random in each MJML run: a reference that
// differs from the new rendering only by them is kept as it is.
const withoutIds = (h) => h && h.replace(/[0-9a-f]{16}/g, 'ID');
for (const [name, source] of docs) {
  const mjmlFile = path.join(outDir, `${name}.mjml`);
  const htmlFile = path.join(outDir, `${name}.html`);
  let html = null;
  try {
    html = (await renderReference(source)).html;
  } catch (err) {
    rejected[name] = String(err.message).split('\n')[0];
  }
  if (html !== null && current(mjmlFile) === source && withoutIds(current(htmlFile)) === withoutIds(html)) html = current(htmlFile);
  const wantHTML = html;
  if (current(mjmlFile) !== source || current(htmlFile) !== wantHTML) {
    stale.push(name);
    if (!check) {
      fs.writeFileSync(mjmlFile, source);
      if (wantHTML === null) {
        if (fs.existsSync(htmlFile)) fs.renameSync(htmlFile, `${htmlFile}.stale`);
      } else {
        fs.writeFileSync(htmlFile, wantHTML);
      }
    }
  }
}
const rejectedFile = path.join(outDir, 'rejected.json');
const sorted = Object.fromEntries(Object.keys(rejected).sort().map((k) => [k, rejected[k]]));
const rejectedText = `${JSON.stringify(sorted, null, 2)}\n`;
if (current(rejectedFile) !== rejectedText) {
  stale.push('rejected.json');
  if (!check) fs.writeFileSync(rejectedFile, rejectedText);
}
const extra = fs.readdirSync(outDir).filter((f) => f.endsWith('.mjml') && !docs.has(f.slice(0, -5)));
for (const f of extra) stale.push(`${f.slice(0, -5)} (not generated any more: move it out)`);
console.log(`MJML ${mjmlVersion}: ${docs.size} documents with a mistake, ${Object.keys(rejected).length} rejected by MJML, ${stale.length} ${check ? 'out of date' : 'written'}`);
for (const s of stale.slice(0, 20)) console.log(`  ${s}`);
process.exitCode = check && stale.length ? 1 : 0;
