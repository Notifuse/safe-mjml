// Writes the MJML documents of MJML's own test suite (packages/mjml/test of
// github.com/mjmlio/mjml, at the tag of the MJML version pinned in
// package.json) into a directory, each next to its reference rendered by that
// version, minified as regenerate.mjs does. TestMJMLTestSuite compares
// safe-mjml with them.
//
// The test files are loaded with mocha's describe and it stubbed and with
// mjml() recording each document it is given (then rendering it with MJML,
// as the test expects), and every test is run; their assertions are not
// checked (chai is a stub): the documents are compared whole instead. The
// files of the command line, the watch mode and mj-include (cli, watch,
// include in their name) are left out: safe-mjml has no command line of
// MJML's, and does not read the file mj-include names. So is
// lazy-head-style, which registers a component of its own through
// mjml-core's JavaScript API, which safe-mjml does not have.
//
// A document is written without a reference when render.mjs cannot render it
// (html-minifier rejects MJML's output), or when it holds htmlmin:ignore
// markers, which html-minifier removes where MJML keeps them: the reference
// would differ from MJML's render.
//
// Usage (from this directory):
//   git clone https://github.com/mjmlio/mjml SRC && git -C SRC checkout v5.4.1
//   node mjml-suite.mjs SRC/packages/mjml/test <output-directory>
//   SAFE_MJML_SUITE_DIR=<output-directory> go test ./mjml -run TestMJMLTestSuite
import fs from 'node:fs';
import path from 'node:path';
import Module, { createRequire } from 'node:module';

import { renderReference } from './render.mjs';

const require = createRequire(import.meta.url);
const [srcDir, outDir] = process.argv.slice(2);
if (!srcDir || !outDir) {
  console.error('usage: node mjml-suite.mjs <mjml-repository>/packages/mjml/test <output-directory>');
  process.exit(2);
}

const mjml = require('mjml');
const recorded = [];
let currentFile = '';
const record = (input, options) => {
  if (typeof input === 'string' && !recorded.some((r) => r.source === input)) {
    recorded.push({ file: currentFile, source: input });
  }
  return mjml(input, { ...options, validationLevel: 'soft' });
};

// chai, and chai-spies, as an object every property and call of which
// returns itself: the assertions are not checked.
const chainable = new Proxy(function chainable() {}, {
  get: (target, prop) => (prop === 'then' ? undefined : chainable),
  apply: () => chainable,
});
const stubs = {
  '../lib': record,
  chai: chainable,
  'chai-spies': chainable,
  '../../mjml-core/lib/index': require('mjml-core'),
};
const originalLoad = Module._load;
Module._load = function load(request, parent, ...rest) {
  if (Object.hasOwn(stubs, request)) return stubs[request];
  if (['cheerio', 'lodash', 'glob'].includes(request)) return originalLoad.call(this, require.resolve(request), parent, ...rest);
  return originalLoad.call(this, request, parent, ...rest);
};

// A test may leave a render unawaited: its failure is the test's.
process.on('unhandledRejection', () => {});

const tests = [];
const context = { timeout() {} };
globalThis.describe = (name, fn) => fn.call(context);
globalThis.it = (name, fn) => tests.push({ file: currentFile, fn });
globalThis.describe.skip = globalThis.it.skip = () => {};
globalThis.before = globalThis.after = globalThis.beforeEach = globalThis.afterEach = () => {};

const files = fs.readdirSync(srcDir).filter((f) => f.endsWith('.test.js') && !/cli|watch|include|lazy-head-style/.test(f)).sort();
for (const file of files) {
  currentFile = file.replace(/\.test\.js$/, '');
  require(path.join(path.resolve(srcDir), file));
}
for (const test of tests) {
  currentFile = test.file;
  try {
    await test.fn.call(context);
  } catch {
    // A test that throws has given its documents all the same.
  }
}

fs.mkdirSync(outDir, { recursive: true });
const counts = {};
let withReference = 0;
const withoutReference = [];
for (const { file, source } of recorded) {
  counts[file] = (counts[file] || 0) + 1;
  const name = `${file}--${counts[file]}`;
  fs.writeFileSync(path.join(outDir, `${name}.mjml`), source);
  if (source.includes('htmlmin:ignore')) {
    withoutReference.push(`${name}: htmlmin:ignore markers, which html-minifier removes and MJML keeps`);
    continue;
  }
  try {
    const { html } = await renderReference(source);
    fs.writeFileSync(path.join(outDir, `${name}.html`), html);
    withReference++;
  } catch (err) {
    withoutReference.push(`${name}: ${String(err.message).split('\n')[0]}`);
  }
}
console.log(`${recorded.length} documents of ${files.length} MJML test files written to ${outDir}, ${withReference} with a reference`);
for (const line of withoutReference) console.log(`  no reference: ${line}`);
