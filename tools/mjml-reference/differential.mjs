// Differential test: renders probe templates with MJML (the version pinned in
// package.json) and with safe-mjml, and prints where their HTML differs.
//
// Usage (from this directory, after `npm install`):
//   node differential.mjs probes/            # every .mjml file of a directory
//   node differential.mjs a.mjml b.mjml      # these files
//   node differential.mjs probes/ --out out/ # also write both renders there
//   node differential.mjs probes/ -q         # one line per probe
//
// It builds safe-mjml from this checkout first. The exit status is 1 when a
// probe differs or renders on one side only.
//
// The method that finds differences: one rule per feature in an inline
// mj-style, each with a color of its own (#b00001, #b00002…), applied to
// elements that name what they test, and groups kept apart in their own
// div so that no rule hides another by specificity. MJML's output is the
// reference, its quirks included. A difference found is reduced to a
// minimal template; once fixed, it becomes a case of mjml/testdata
// (regenerate.mjs) or, if it stays, a line of docs/differences-from-mjml.md.
//
// The comparison ignores what MJML's minifier changes without meaning
// (attribute order, declaration order and spacing, empty attributes and
// declarations, whitespace, "/>" outside svg and math, boolean attributes
// written in full, colgroup tags) and the random IDs of mj-carousel and
// mj-navbar. It reads tags with a pattern, not an HTML parser: markup that
// breaks the tag syntax (a ">" inside an attribute, a tag left open) can show
// as a difference where the integration tests, which parse the DOM, see
// none. On mjml/testdata it reports the cases docs/compatibility.md
// explains, and two such artifacts: ending-tags-quoted-and-comments and
// mj-attribute-angle-brackets.
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const mjml2html = require('mjml');

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');

const args = process.argv.slice(2);
const quiet = args.includes('-q');
const outIndex = args.indexOf('--out');
const outDir = outIndex >= 0 ? path.resolve(args[outIndex + 1]) : null;
const inputs = args.filter((a, i) => !a.startsWith('-') && (outIndex < 0 || i !== outIndex + 1));
if (inputs.length === 0) {
  console.error('usage: node differential.mjs <dir or .mjml files> [--out dir] [-q]');
  process.exit(2);
}

const files = inputs.flatMap((p) => {
  const abs = path.resolve(p);
  if (fs.statSync(abs).isDirectory()) {
    return fs.readdirSync(abs).filter((f) => f.endsWith('.mjml')).sort().map((f) => path.join(abs, f));
  }
  return [abs];
});

const binDir = fs.mkdtempSync(path.join(os.tmpdir(), 'safe-mjml-differential-'));
process.on('exit', () => fs.rmSync(binDir, { recursive: true, force: true }));
const bin = path.join(binDir, 'safe-mjml');
const build = spawnSync('go', ['build', '-o', bin, './cmd/safe-mjml'], { cwd: root, encoding: 'utf8' });
if (build.status !== 0) {
  console.error(build.stderr);
  process.exit(2);
}
if (outDir) fs.mkdirSync(outDir, { recursive: true });

function renderMJML(file) {
  // mjml-core warns on every render that minify is for its CLI.
  const warn = console.warn;
  console.warn = () => {};
  try {
    const r = mjml2html(fs.readFileSync(file, 'utf8'), { minify: true, validationLevel: 'soft', filePath: file });
    return { html: r.html };
  } catch (err) {
    return { err: String(err.message || err).split('\n')[0] };
  } finally {
    console.warn = warn;
  }
}

function renderSafeMJML(file) {
  const r = spawnSync(bin, ['compile', file], { encoding: 'utf8', maxBuffer: 64 << 20 });
  if (r.status !== 0) return { err: (r.stderr || '').trim().split('\n').pop() };
  return { html: r.stdout };
}

// Tokens: tags with sorted, normalized attributes, comments, and text with
// its whitespace collapsed.
const RANDOM_ID = /[0-9a-f]{16}(?![0-9a-f])/g;
const TOKEN = /<!--[\s\S]*?-->|<[^>]*>|[^<]+/g;
const ATTR = /([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g;
const DROP_EMPTY = new Set(['class', 'id', 'style', 'title', 'lang', 'dir']);
const BOOLEAN = new Set('checked,compact,declare,defer,disabled,ismap,multiple,nohref,noresize,noshade,nowrap,readonly,selected'.split(','));

function normalizeTag(tag) {
  const m = /^<([a-zA-Z][^\s/>]*)([\s\S]*?)(\/?)>$/.exec(tag);
  if (!m) return tag;
  const attrs = [];
  for (const a of m[2].matchAll(ATTR)) {
    const name = a[1];
    let value = a[2] ?? a[3] ?? a[4];
    if (value !== undefined) value = value.replaceAll('&quot;', '"');
    if ((value === undefined || value.trim() === '') && (DROP_EMPTY.has(name) || name.startsWith('on'))) continue;
    if (value === undefined && BOOLEAN.has(name)) value = name;
    if (name === 'class' && value !== undefined) value = value.split(/\s+/).filter(Boolean).join(' ');
    if (name === 'style' && value !== undefined) value = normalizeDeclarations(value);
    if (name === 'value' && !value && m[1].toLowerCase() === 'input') continue;
    attrs.push(value === undefined ? name : `${name}="${value}"`);
  }
  attrs.sort();
  return `<${m[1]}${attrs.length ? ' ' + attrs.join(' ') : ''}${m[3]}>`;
}

// normalizeDeclarations sorts the declarations of a style attribute and
// writes them without spaces: their order and spacing carry no meaning.
function normalizeDeclarations(style) {
  return style
    .split(';')
    .map((d) => d.trim().replace(/\s*:\s*/, ':').replace(/\s*!\s*important$/i, '!important').replace(/\s+/g, ' '))
    .filter((d) => d && !d.endsWith(':'))
    .sort()
    .join(';');
}

// normalizeCSS writes a stylesheet without the spaces CSS ignores.
function normalizeCSS(css) {
  return css.replace(/\s+/g, ' ').replace(/\s*([{}:;,>+~!])\s*/g, '$1').replace(/;}/g, '}').trim();
}

// normalizeComment normalizes the tags inside a comment, such as Outlook's
// conditional comments, and collapses its whitespace.
function normalizeComment(comment) {
  return comment.replace(/<[a-zA-Z][^<>]*>/g, normalizeTag).replace(/\s+/g, ' ').replace(/> </g, '><');
}

// tokens reads a document as tokens. Outside svg and math, a "/" before ">"
// means nothing in HTML and is dropped; inside them it closes the element.
function tokens(html) {
  const out = [];
  let inStyle = false;
  let foreign = 0;
  for (const [t] of html.replace(RANDOM_ID, 'ID').matchAll(TOKEN)) {
    const s = t.trim();
    if (!s || s === '<colgroup>' || s === '</colgroup>') continue;
    const tag = /^<\/?([a-zA-Z]+)/.exec(s)?.[1]?.toLowerCase();
    if (tag === 'svg' || tag === 'math') {
      if (s.startsWith('</')) foreign--;
      else if (!s.endsWith('/>')) foreign++;
    }
    if (s.startsWith('<!--')) out.push(normalizeComment(s));
    else if (s.startsWith('</')) out.push(s.toLowerCase().replace(/\s+/g, ''));
    else if (s.startsWith('<') && foreign === 0) out.push(normalizeTag(s).replace(/\/>$/, '>'));
    else if (s.startsWith('<')) out.push(normalizeTag(s));
    else if (inStyle) out.push(normalizeCSS(s));
    else out.push(s.split(/\s+/).join(' '));
    if (s.startsWith('<style')) inStyle = true;
    else if (s.startsWith('</style')) inStyle = false;
  }
  return out;
}

// diff returns the differing runs of two token lists, from their longest
// common subsequence.
function diff(a, b) {
  const n = a.length;
  const m = b.length;
  if (n * m > 4e7) return [{ at: '(too long to align)', mjml: a.slice(0, 5), safe: b.slice(0, 5) }];
  const lcs = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }
  const runs = [];
  let i = 0;
  let j = 0;
  let run = null;
  const flush = () => {
    if (run) runs.push(run);
    run = null;
  };
  while (i < n || j < m) {
    if (i < n && j < m && a[i] === b[j]) {
      flush();
      i++;
      j++;
    } else {
      run ??= { at: i > 0 ? a[i - 1] : '', mjml: [], safe: [] };
      if (j < m && (i === n || lcs[i][j + 1] >= lcs[i + 1][j])) run.safe.push(b[j++]);
      else run.mjml.push(a[i++]);
    }
  }
  flush();
  return runs;
}

let failed = 0;
for (const file of files) {
  const name = path.basename(file, '.mjml');
  const want = renderMJML(file);
  const got = renderSafeMJML(file);
  if (outDir) {
    fs.writeFileSync(path.join(outDir, `${name}.mjml.html`), want.html ?? `ERROR ${want.err}\n`);
    fs.writeFileSync(path.join(outDir, `${name}.safe-mjml.html`), got.html ?? `ERROR ${got.err}\n`);
  }
  if (want.err || got.err) {
    if (want.err && got.err) {
      console.log(`${name}: both refuse it`);
      continue;
    }
    failed++;
    console.log(`${name}: ERROR in ${want.err ? 'MJML' : 'safe-mjml'}: ${want.err ?? got.err}`);
    continue;
  }
  const runs = diff(tokens(want.html), tokens(got.html));
  if (runs.length === 0) {
    if (!quiet) console.log(`${name}: same`);
    continue;
  }
  failed++;
  console.log(`${name}: DIFFERENT`);
  if (quiet) continue;
  for (const r of runs) {
    console.log(`  after ${r.at.slice(0, 150)}`);
    for (const t of r.mjml) console.log(`    - MJML:      ${t.slice(0, 400)}`);
    for (const t of r.safe) console.log(`    + safe-mjml: ${t.slice(0, 400)}`);
  }
}
console.log(`${files.length} probes, ${failed} different`);
process.exit(failed ? 1 : 0);
