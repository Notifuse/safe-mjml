// Passes every output of a library through html-minifier with the options
// of the reference pipeline (tools/mjml-reference/render.mjs), so that all
// libraries are compared on the same footing as the references.
const path = require('path');
const fs = require('fs');
const req = require('module').createRequire(path.join(__dirname, '..', 'mjml-reference', 'package.json'));
const { minify } = req('html-minifier');
const [inDir, outDir] = process.argv.slice(2);
fs.mkdirSync(outDir, { recursive: true });
let failed = 0;
for (const f of fs.readdirSync(inDir)) {
  if (!f.endsWith('.html')) continue;
  const html = fs.readFileSync(path.join(inDir, f), 'utf8');
  let out;
  try {
    out = minify(html, { collapseWhitespace: true, minifyCSS: false, caseSensitive: true, removeEmptyAttributes: true });
  } catch (e) {
    failed++;
    out = html;
  }
  fs.writeFileSync(path.join(outDir, f), out);
}
console.log(`${inDir}: ${failed} outputs the minifier could not parse (kept as written)`);
