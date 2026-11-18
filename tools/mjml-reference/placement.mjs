// Generates mjml/testdata/placement: one document for each pair of a body
// component and a container it can be written in, allowed or not, and the
// reference HTML MJML renders for it, through render.mjs as regenerate.mjs.
//
// The hand-written cases put a component in the wrong parent only where a
// bug had been found (8 pairs). An mj-group outside a section wrote an
// Outlook cell MJML does not write, and every child of an mj-group but its
// columns and raw elements was dropped without an error: no document had
// either shape. MJML lists the children it allows in
// mjml-preset-core/lib/dependencies.js; it renders the others with a
// warning. Each pair is written twice: the child alone in its parent, and
// after a sibling the parent allows, since a parent shares its width among
// its children. TestPlacement checks that safe-mjml renders each document as
// MJML does, or returns an error; never anything else.
//
// Usage (from this directory, after `npm install`):
//   node placement.mjs           # (re)write every document, its reference and rejected.json
//   node placement.mjs --check   # report documents or references that are out of date
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { renderReference, mjmlVersion } from './render.mjs';

const require = createRequire(import.meta.url);
const dependencies = require('mjml-preset-core/lib/dependencies.js');

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.resolve(here, '../../mjml/testdata/placement');

// Each body component, with a text of its own wherever it writes one, so
// that a dropped child shows. Non-round widths, as mjml/testdata/AGENTS.md
// asks.
const CHILDREN = {
  'mj-section': '<mj-section padding="0 7px"><mj-column><mj-text>SECTION</mj-text></mj-column></mj-section>',
  'mj-wrapper': '<mj-wrapper padding="3px 13px"><mj-section><mj-column><mj-text>WRAPPER</mj-text></mj-column></mj-section></mj-wrapper>',
  'mj-hero': '<mj-hero padding="5px 11px"><mj-text>HERO</mj-text></mj-hero>',
  'mj-column': '<mj-column width="41%"><mj-text>COLUMN</mj-text></mj-column>',
  'mj-group': '<mj-group width="77%"><mj-column><mj-text>GROUP-1</mj-text></mj-column><mj-column width="33.33%"><mj-text>GROUP-2</mj-text></mj-column></mj-group>',
  'mj-raw': '<mj-raw><p>RAW</p></mj-raw>',
  'mj-text': '<mj-text>TEXT</mj-text>',
  'mj-button': '<mj-button href="#">BUTTON</mj-button>',
  'mj-image': '<mj-image src="https://example.com/a.png" alt="IMAGE" />',
  'mj-divider': '<mj-divider width="61%" />',
  'mj-spacer': '<mj-spacer height="7px" />',
  'mj-table': '<mj-table><tr><td>TABLE</td></tr></mj-table>',
  'mj-social': '<mj-social><mj-social-element name="facebook" href="#">SOCIAL</mj-social-element></mj-social>',
  'mj-social-element': '<mj-social-element name="twitter" href="#">SOCIAL-ELEMENT</mj-social-element>',
  'mj-navbar': '<mj-navbar><mj-navbar-link href="#">NAVBAR</mj-navbar-link></mj-navbar>',
  'mj-navbar-link': '<mj-navbar-link href="#">NAVBAR-LINK</mj-navbar-link>',
  'mj-accordion': '<mj-accordion><mj-accordion-element><mj-accordion-title>ACCORDION-TITLE</mj-accordion-title><mj-accordion-text>ACCORDION-TEXT</mj-accordion-text></mj-accordion-element></mj-accordion>',
  'mj-accordion-element': '<mj-accordion-element><mj-accordion-title>ELEMENT-TITLE</mj-accordion-title><mj-accordion-text>ELEMENT-TEXT</mj-accordion-text></mj-accordion-element>',
  'mj-accordion-title': '<mj-accordion-title>TITLE</mj-accordion-title>',
  'mj-accordion-text': '<mj-accordion-text>ACC-TEXT</mj-accordion-text>',
  'mj-carousel': '<mj-carousel><mj-carousel-image src="https://example.com/c.png" alt="CAROUSEL" /></mj-carousel>',
  'mj-carousel-image': '<mj-carousel-image src="https://example.com/d.png" alt="CAROUSEL-IMAGE" />',
};

// Each container, as the document around its content, and a sibling it
// allows (null for the parents of whom no sibling is a separate case).
const body = (content) => `<mjml>\n  <mj-body width="601px">\n    ${content}\n  </mj-body>\n</mjml>\n`;
const inColumn = (content) => body(`<mj-section padding="0 9px"><mj-column width="347px">${content}</mj-column></mj-section>`);
const PARENTS = {
  'mj-body': { wrap: body, sibling: '<mj-section><mj-column><mj-text>SIBLING</mj-text></mj-column></mj-section>' },
  'mj-wrapper': { wrap: (c) => body(`<mj-wrapper padding="3px 13px">${c}</mj-wrapper>`), sibling: '<mj-section><mj-column><mj-text>SIBLING</mj-text></mj-column></mj-section>' },
  'mj-section': { wrap: (c) => body(`<mj-section padding="0 9px">${c}</mj-section>`), sibling: '<mj-column width="29%"><mj-text>SIBLING</mj-text></mj-column>' },
  'mj-group': { wrap: (c) => body(`<mj-section><mj-group width="83%">${c}</mj-group></mj-section>`), sibling: '<mj-column width="29%"><mj-text>SIBLING</mj-text></mj-column>' },
  'mj-column': { wrap: inColumn, sibling: '<mj-text>SIBLING</mj-text>' },
  'mj-hero': { wrap: (c) => body(`<mj-hero padding="5px 11px">${c}</mj-hero>`), sibling: '<mj-text>SIBLING</mj-text>' },
  'mj-accordion': { wrap: (c) => inColumn(`<mj-accordion>${c}</mj-accordion>`), sibling: '<mj-accordion-element><mj-accordion-title>SIBLING-TITLE</mj-accordion-title><mj-accordion-text>SIBLING-TEXT</mj-accordion-text></mj-accordion-element>' },
  'mj-accordion-element': { wrap: (c) => inColumn(`<mj-accordion><mj-accordion-element>${c}</mj-accordion-element></mj-accordion>`), sibling: '<mj-accordion-title>SIBLING-TITLE</mj-accordion-title>' },
  'mj-navbar': { wrap: (c) => inColumn(`<mj-navbar>${c}</mj-navbar>`), sibling: '<mj-navbar-link href="#">SIBLING</mj-navbar-link>' },
  'mj-social': { wrap: (c) => inColumn(`<mj-social>${c}</mj-social>`), sibling: '<mj-social-element name="facebook" href="#">SIBLING</mj-social-element>' },
  'mj-carousel': { wrap: (c) => inColumn(`<mj-carousel>${c}</mj-carousel>`), sibling: '<mj-carousel-image src="https://example.com/s.png" alt="SIBLING" />' },
};

// Every parent and child must be one MJML knows: a renamed component would
// otherwise leave its pairs out without a word.
const known = new Set([...Object.keys(dependencies), ...Object.values(dependencies).flat().filter((t) => typeof t === 'string')]);
for (const tag of Object.keys(CHILDREN)) {
  if (!known.has(tag)) throw new Error(`${tag} is not in mjml-preset-core's dependencies`);
}
for (const tag of Object.keys(PARENTS)) {
  if (!(tag in dependencies)) throw new Error(`${tag} has no children list in mjml-preset-core's dependencies`);
}

function documents() {
  const out = new Map();
  for (const [parent, { wrap, sibling }] of Object.entries(PARENTS)) {
    for (const [child, markup] of Object.entries(CHILDREN)) {
      const allowed = dependencies[parent].includes(child) ? 'allowed' : 'misplaced';
      const name = `${child.slice(3)}-in-${parent.slice(3)}--${allowed}`;
      out.set(name, wrap(markup));
      out.set(`${name}--after-sibling`, wrap(sibling + markup));
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
  if (current(mjmlFile) !== source || current(htmlFile) !== html) {
    stale.push(name);
    if (!check) {
      fs.writeFileSync(mjmlFile, source);
      if (html === null) {
        if (fs.existsSync(htmlFile)) fs.rmSync(htmlFile);
      } else {
        fs.writeFileSync(htmlFile, html);
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
for (const f of extra) stale.push(`${f.slice(0, -5)} (not generated any more: remove it)`);
console.log(`MJML ${mjmlVersion}: ${docs.size} placements, ${Object.keys(rejected).length} rejected by MJML, ${stale.length} ${check ? 'out of date' : 'written'}`);
for (const s of stale.slice(0, 20)) console.log(`  ${s}`);
process.exitCode = check && stale.length ? 1 : 0;
