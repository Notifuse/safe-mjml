// Writes MJML's official email templates (github.com/mjmlio/email-templates,
// at the commit below) into a directory, each next to its reference
// rendered by the MJML version pinned in package.json, minified as
// regenerate.mjs does. TestOfficialTemplates compares safe-mjml with them.
//
// The repository has no license, so its templates are not copied into
// safe-mjml: the CI clones it at OFFICIAL_COMMIT and runs this script.
//
// Usage (from this directory):
//   git clone https://github.com/mjmlio/email-templates SRC && git -C SRC checkout OFFICIAL_COMMIT
//   node official.mjs SRC/templates <output-directory>
//   SAFE_MJML_OFFICIAL_DIR=<output-directory> go test ./mjml -run TestOfficialTemplates
//
// OFFICIAL_COMMIT: a7230b13086c1746524839768fe0bc15a897d960 (2026-09-16), also in .github/workflows/test.yml.
import fs from 'node:fs';
import path from 'node:path';

import { renderReference } from './render.mjs';

const [srcDir, outDir] = process.argv.slice(2);
if (!srcDir || !outDir) {
  console.error('usage: node official.mjs <templates-directory> <output-directory>');
  process.exit(2);
}
fs.mkdirSync(outDir, { recursive: true });
let written = 0;
const failed = [];
for (const file of fs.readdirSync(srcDir).filter((f) => f.endsWith('.mjml')).sort()) {
  const source = fs.readFileSync(path.join(srcDir, file), 'utf8');
  let html;
  try {
    html = (await renderReference(source, { filePath: path.join(srcDir, file) })).html;
  } catch (err) {
    failed.push(`${file}: ${String(err.message).split('\n')[0]}`);
    continue;
  }
  fs.writeFileSync(path.join(outDir, file), source);
  fs.writeFileSync(path.join(outDir, `${file.slice(0, -5)}.html`), html);
  written++;
}
console.log(`${written} official templates written to ${outDir}`);
for (const f of failed) console.log(`! MJML failed: ${f}`);
process.exitCode = failed.length ? 1 : 0;
