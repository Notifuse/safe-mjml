// Regenerates mjml/testdata/<name>.html from <name>.mjml with the MJML version
// pinned in package.json, in the format the integration tests expect
// (minified HTML, as `mjml --minify` writes it).
//
// Usage (from this directory, after `npm install`):
//   node regenerate.mjs              # every .mjml file in mjml/testdata
//   node regenerate.mjs basic mj-hero   # only these test cases
//   node regenerate.mjs --check      # report references that differ, write nothing
//
// mj-carousel and mj-navbar contain random 16-hex-digit IDs that change on every
// MJML run. The integration tests give each render the IDs of its own
// reference, so any IDs work; to keep regenerated references from changing
// for nothing, when a reference already exists, the IDs MJML generated are
// replaced, in order of appearance, by those of the existing reference.
// referenceRandomID in mjml/id_generator_test.go reads the IDs back with the
// same rule, minus the "#" alternative, which no output uses; keep them in step.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const mjml2html = require('mjml');
const mjmlVersion = require('mjml/package.json').version;

const here = path.dirname(fileURLToPath(import.meta.url));
const testdata = path.resolve(here, '../../mjml/testdata');

// Inputs MJML rejects ("Malformed MJML"): their reference is kept as is.
// - mjml: deliberately malformed document.
// - mj-raw-head: has no mj-body, which MJML 4.18 refuses to render.
const SKIP = new Set(['mjml', 'mj-raw-head']);

const RANDOM_ID = /(?<=mj-carousel-(?:radio-)?|id="|for="|#)[0-9a-f]{16}(?![0-9a-f])/g;

function distinctInOrder(html) {
  const seen = [];
  for (const m of html.matchAll(RANDOM_ID)) {
    if (!seen.includes(m[0])) seen.push(m[0]);
  }
  return seen;
}

function keepExistingIDs(generated, existing) {
  const fresh = distinctInOrder(generated);
  if (fresh.length === 0 || existing === null) return { html: generated };
  const previous = distinctInOrder(existing);
  if (previous.length !== fresh.length) {
    return { html: generated, warning: `found ${fresh.length} random IDs, existing reference has ${previous.length}; IDs left as generated` };
  }
  const map = new Map(fresh.map((id, i) => [id, previous[i]]));
  return { html: generated.replace(RANDOM_ID, (id) => map.get(id) ?? id) };
}

const args = process.argv.slice(2);
const check = args.includes('--check');
const names = args.filter((a) => !a.startsWith('--'));
const list = names.length
  ? names
  : fs.readdirSync(testdata).filter((f) => f.endsWith('.mjml')).map((f) => f.slice(0, -5)).sort();

const changed = [];
const problems = [];
for (const name of list) {
  if (SKIP.has(name)) continue;
  const input = path.join(testdata, `${name}.mjml`);
  const output = path.join(testdata, `${name}.html`);
  const existing = fs.existsSync(output) ? fs.readFileSync(output, 'utf8') : null;
  let result;
  try {
    result = mjml2html(fs.readFileSync(input, 'utf8'), { minify: true, validationLevel: 'soft', filePath: input });
  } catch (err) {
    problems.push(`${name}: MJML failed: ${err.message}`);
    continue;
  }
  const { html, warning } = keepExistingIDs(result.html, existing);
  if (warning) problems.push(`${name}: ${warning}`);
  if (html !== existing) {
    changed.push(name);
    if (!check) fs.writeFileSync(output, html);
  }
}

console.log(`MJML ${mjmlVersion}: ${list.length} test cases, ${changed.length} ${check ? 'differ' : 'updated'}`);
for (const name of changed) console.log(`  ${name}`);
for (const p of problems) console.log(`! ${p}`);
// In check mode, a reference that differs and an input MJML cannot render
// (outside SKIP) are both failures.
process.exitCode = check && (changed.length || problems.some((p) => p.includes('MJML failed'))) ? 1 : 0;
