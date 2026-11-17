// The reference pipeline: MJML's unminified render, compacted with
// html-minifier 4.0.0 and the options MJML 4.18 passed it for minify: true.
//
// MJML 5 minifies with htmlnano and cssnano, which rewrite the CSS of every
// document (`Ubuntu,Helvetica`, `rgb(1,2,3)`, no final `;`) and throw on CSS
// they cannot parse: its minified output cannot be compared with safe-mjml's.
// Its unminified output cannot either, since the comparison relies on the
// minifier dropping empty `style=""` and collapsing whitespace. Applied to
// MJML 4.18's unminified output, this pipeline gives the references MJML 4.18
// wrote with minify: true, byte for byte (docs/mjml-5.md).
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const mjml2html = require('mjml');
const { minify } = require('html-minifier');

export const mjmlVersion = require('mjml/package.json').version;

export const MINIFY_OPTIONS = Object.freeze({
  collapseWhitespace: true,
  minifyCSS: false,
  caseSensitive: true,
  removeEmptyAttributes: true,
});

// renderReference renders source as the tests expect it. It throws when MJML
// or html-minifier fails on the document, and returns MJML's validation
// errors with the HTML.
export async function renderReference(source, options = {}) {
  const result = await mjml2html(source, { validationLevel: 'soft', ...options, minify: false });
  return { html: minify(result.html, MINIFY_OPTIONS), errors: result.errors };
}
