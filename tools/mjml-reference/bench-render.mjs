// Renders each template given with MJML 4.18 in process, warm, and prints
// "name<TAB>milliseconds per render" (the mean of the timed renders), for
// tools/bench-vs-mjml.sh.
//
//   node bench-render.mjs RENDERS FILE...
import fs from 'node:fs';
import path from 'node:path';
import mjml2html from 'mjml';

const [renders, ...files] = process.argv.slice(2);
const count = Number(renders);
for (const file of files) {
  const source = fs.readFileSync(file, 'utf8');
  const options = { validationLevel: 'soft', filePath: file };
  for (let i = 0; i < 50; i++) mjml2html(source, options); // warm up
  const start = process.hrtime.bigint();
  for (let i = 0; i < count; i++) mjml2html(source, options);
  const ms = Number(process.hrtime.bigint() - start) / 1e6 / count;
  console.log(`${path.basename(file, '.mjml')}\t${ms.toFixed(3)}`);
}
