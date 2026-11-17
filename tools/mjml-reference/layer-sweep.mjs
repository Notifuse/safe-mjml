// Writes the layer sweep into a directory: for every component and every
// attribute MJML allows on it, documents that set the attribute in one or
// several layers (element, mj-class, mj-attributes tag, mj-all, parent,
// grandparent), with values and empty values (layers.mjs), each next to its
// reference rendered by the MJML version pinned in package.json.
//
// About 6,800 documents: too many to commit. tools/release-diff.sh renders
// them with the previous release and the working tree when SWEEP=1, and fails
// on a document that the working tree renders further from MJML.
//
// Usage (from this directory): node layer-sweep.mjs <output-directory> [tag-regexp]
import fs from 'node:fs';
import path from 'node:path';
import { CONTEXTS, components, layerCases, layeredDocument, sampleValues } from './layers.mjs';

import { renderReference } from './render.mjs';

const [outDir, only] = process.argv.slice(2);
if (!outDir) {
  console.error('usage: node layer-sweep.mjs <output-directory> [tag-regexp]');
  process.exit(2);
}
fs.mkdirSync(outDir, { recursive: true });
const filter = only ? new RegExp(only) : null;
let written = 0;
const seen = new Set();
let rejected = 0;
for (const tag of Object.keys(CONTEXTS)) {
  if (filter && !filter.test(tag)) continue;
  for (const [attr, type] of Object.entries(components[tag].allowedAttributes || {})) {
    const values = sampleValues(attr, type);
    if (!values) continue;
    for (const [name, layers] of Object.entries(layerCases(tag, values))) {
      const source = layeredDocument(tag, attr, layers);
      let html;
      try {
        html = (await renderReference(source)).html;
      } catch {
        rejected++; // MJML rejects it: nothing to compare with
        continue;
      }
      const base = path.join(outDir, `${tag.slice(3)}__${attr}__${name}`);
      if (seen.has(base.toLowerCase())) throw new Error(`two sweep documents would share the file ${base}`);
      seen.add(base.toLowerCase());
      fs.writeFileSync(`${base}.mjml`, source);
      fs.writeFileSync(`${base}.html`, html);
      written++;
    }
  }
}
console.log(`${written} sweep documents written to ${outDir}${rejected ? `, ${rejected} rejected by MJML` : ''}`);
