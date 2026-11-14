// Shared by matrix.mjs and layer-sweep.mjs: documents that set one attribute
// of one component in one or several of the layers MJML resolves, the element,
// mj-class, the mj-attributes tag, mj-all, and the parent and grandparent that
// pass attributes down, with values or the empty string.
//
// Adapted from the generator of the review of refactor step 3, which
// found readers that branched on the value MJML does not test (mode ==
// "vertical" where mjml-social tests === 'horizontal') and only failed when an
// empty value over a lower layer reached them.
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
require('mjml');
export const { components } = require('mjml-core');

// Where each component sits: the path from mj-body to it, its content, and
// what stands before or after it in its parent.
export const CONTEXTS = {
  'mj-body': { path: ['mj-body'], inner: '<mj-section><mj-column><mj-text>Hi</mj-text></mj-column></mj-section>' },
  'mj-section': { path: ['mj-body', 'mj-section'], inner: '<mj-column><mj-text>Hi</mj-text></mj-column>' },
  'mj-wrapper': { path: ['mj-body', 'mj-wrapper'], inner: '<mj-section><mj-column><mj-text>Hi</mj-text></mj-column></mj-section>' },
  'mj-hero': { path: ['mj-body', 'mj-hero'], inner: '<mj-text>Hi</mj-text>' },
  'mj-column': { path: ['mj-body', 'mj-section', 'mj-column'], inner: '<mj-text>Hi</mj-text>', after: '<mj-column><mj-text>B</mj-text></mj-column>' },
  'mj-group': { path: ['mj-body', 'mj-section', 'mj-group'], inner: '<mj-column><mj-text>Hi</mj-text></mj-column><mj-column><mj-text>B</mj-text></mj-column>', after: '<mj-column><mj-text>C</mj-text></mj-column>' },
  'mj-text': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-text'], inner: 'Hello <b>w</b>' },
  'mj-button': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-button'], inner: 'Click' },
  'mj-image': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-image'], inner: '', base: { src: 'https://example.com/i.png' } },
  'mj-divider': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-divider'], inner: '' },
  'mj-spacer': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-spacer'], inner: '' },
  'mj-table': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-table'], inner: '<tr><td>a</td><td>b</td></tr>' },
  'mj-social': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-social'], inner: '<mj-social-element name="facebook" href="https://example.com/f">FB</mj-social-element><mj-social-element name="x" href="https://example.com/x">X</mj-social-element>' },
  'mj-social-element': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-social', 'mj-social-element'], inner: 'FB', base: { name: 'facebook', href: 'https://example.com/f' }, after: '<mj-social-element name="github" href="https://example.com/g">GH</mj-social-element>' },
  'mj-navbar': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-navbar'], inner: '<mj-navbar-link href="/a">A</mj-navbar-link><mj-navbar-link href="/b">B</mj-navbar-link>' },
  'mj-navbar-link': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-navbar', 'mj-navbar-link'], inner: 'A', base: { href: '/a' }, after: '<mj-navbar-link href="/b">B</mj-navbar-link>' },
  'mj-accordion': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-accordion'], inner: '<mj-accordion-element><mj-accordion-title>T</mj-accordion-title><mj-accordion-text>X</mj-accordion-text></mj-accordion-element><mj-accordion-element><mj-accordion-title>T2</mj-accordion-title></mj-accordion-element>' },
  'mj-accordion-element': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-accordion', 'mj-accordion-element'], inner: '<mj-accordion-title>T</mj-accordion-title><mj-accordion-text>X</mj-accordion-text>', after: '<mj-accordion-element><mj-accordion-text>only text</mj-accordion-text></mj-accordion-element>' },
  'mj-accordion-title': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-accordion', 'mj-accordion-element', 'mj-accordion-title'], inner: 'T', after: '<mj-accordion-text>X</mj-accordion-text>' },
  'mj-accordion-text': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-accordion', 'mj-accordion-element', 'mj-accordion-text'], inner: 'X', before: '<mj-accordion-title>T</mj-accordion-title>' },
  'mj-carousel': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-carousel'], inner: '<mj-carousel-image src="https://example.com/1.png"/><mj-carousel-image src="https://example.com/2.png"/>' },
  'mj-carousel-image': { path: ['mj-body', 'mj-section', 'mj-column', 'mj-carousel', 'mj-carousel-image'], inner: '', base: { src: 'https://example.com/1.png' }, after: '<mj-carousel-image src="https://example.com/2.png"/>' },
};

// Components whose children get attributes from them.
export const PASSING_PARENTS = new Set(['mj-social', 'mj-navbar', 'mj-accordion', 'mj-accordion-element', 'mj-carousel']);

// enumValues returns the values of an enum attribute type, or null.
export function enumValues(type) {
  const m = /^enum\((.*)\)$/.exec(type);
  return m ? m[1].split(',') : null;
}

// sampleValues returns four distinct values of an attribute, or null for the
// attributes the sweep leaves out.
export function sampleValues(attr, type) {
  if (attr === 'mj-class' || attr === 'name') return null;
  if (attr === 'font-family') return ['Roboto, Arial', 'Ubuntu, sans-serif', 'Lato', 'Open Sans, Helvetica'];
  if (attr === 'css-class') return ['k1', 'k2', 'k3', 'k4'];
  if (/url|src$|^src|href|srcset/.test(attr)) return ['https://example.com/v1.png', 'https://example.com/v2.png', 'https://example.com/v3.png', 'https://example.com/v4.png'];
  if (/border$|^(inner-)?border-(top|bottom|left|right)$/.test(attr)) return ['3px solid #ff0001', '1px dashed #00ff02', '5px dotted #0000f3', '2px solid #ab12cd'];
  if (attr === 'font-size') return ['11px', '13px', '17px', '19px'];
  if (attr === 'font-weight') return ['bold', '300', '700', 'normal'];
  if (attr === 'font-style') return ['italic', 'normal', 'oblique', 'italic'];
  if (attr === 'text-decoration') return ['underline', 'none', 'overline', 'line-through'];
  if (attr === 'text-transform') return ['uppercase', 'lowercase', 'capitalize', 'none'];
  if (attr === 'line-height') return ['17px', '1.3', '23px', '150%'];
  if (attr === 'letter-spacing') return ['1px', '2px', '0.3em', '-1px'];
  if (attr === 'background-position') return ['top left', 'center center', '10px 20px', 'bottom right'];
  if (attr === 'background-size') return ['cover', 'contain', 'auto', '50%'];
  if (attr === 'border-style') return ['dashed', 'dotted', 'solid', 'double'];
  if (attr === 'sizes') return ['(max-width: 300px) 100vw', '50vw', '33vw', '25vw'];
  const e = enumValues(type);
  if (e) return [0, 1, 2, 3].map((i) => e[i % e.length]);
  if (type === 'color') return ['#ff0001', '#0f2', '#0000F3', 'rgb(10, 20, 30)'];
  if (type === 'boolean') return ['true', 'false', 'true', 'false'];
  if (type === 'integer') return ['3', '5', '7', '9'];
  if (/\{1,4\}/.test(type)) return ['7px 13px', '3px 5px 9px 11px', '17px', '1px 2px 3px'];
  if (/^unitWithNegative/.test(type)) return ['1px', '2px', '-1px', '3px'];
  if (/^unit\(/.test(type)) return /%/.test(type) ? ['77px', '31%', '123px', '45%'] : ['21px', '33px', '47px', '59px'];
  return ['v1', 'v2', 'v3', 'v4'];
}

const escape = (s) => String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;');
const attributes = (o) => Object.entries(o).map(([k, v]) => ` ${k}="${escape(v)}"`).join('');

// layeredDocument builds the document that sets attr on tag in the given
// layers: e (element), c (mj-class), t (mj-attributes tag), a (mj-all),
// p (the parent element), pT (the parent's mj-attributes tag), g (the
// grandparent element). A layer left undefined is not set; '' sets it empty.
export function layeredDocument(tag, attr, layers) {
  const ctx = CONTEXTS[tag];
  const path = ctx.path;
  const head = [];
  if (layers.a !== undefined) head.push(`<mj-all${attributes({ [attr]: layers.a })}/>`);
  if (layers.t !== undefined) head.push(`<${tag}${attributes({ [attr]: layers.t })}/>`);
  if (layers.c !== undefined) head.push(`<mj-class name="cl"${attributes({ [attr]: layers.c })}/>`);
  if (layers.pT !== undefined) head.push(`<${path[path.length - 2]}${attributes({ [attr]: layers.pT })}/>`);
  let body = '';
  let close = '';
  for (let i = 0; i < path.length; i++) {
    const t = path[i];
    const a = {};
    if (t === 'mj-body') a.width = '601px';
    if (i === path.length - 1) {
      Object.assign(a, ctx.base || {});
      if (layers.c !== undefined) a['mj-class'] = 'cl';
      if (layers.e !== undefined) a[attr] = layers.e;
      body += (ctx.before || '') + `<${t}${attributes(a)}>${ctx.inner}</${t}>` + (ctx.after || '');
    } else {
      if (i === path.length - 2 && layers.p !== undefined) a[attr] = layers.p;
      if (i === path.length - 3 && layers.g !== undefined) a[attr] = layers.g;
      body += `<${t}${attributes(a)}>`;
      close = `</${t}>` + close;
    }
  }
  return `<mjml><mj-head><mj-attributes>${head.join('')}</mj-attributes></mj-head>${body}${close}</mjml>\n`;
}

// layerCases returns, for an attribute with four sample values, the layer
// combinations of the full sweep, keyed by a short name (names differ by
// more than case: they become file names, on case-insensitive file systems
// too).
export function layerCases(tag, [v1, v2, v3, v4]) {
  const cases = {
    e: { e: v1 }, e0: { e: '' }, c: { c: v2 }, c0: { c: '' }, t: { t: v3 }, t0: { t: '' }, a: { a: v4 }, a0: { a: '' },
    ecta: { e: v1, c: v2, t: v3, a: v4 }, cta: { c: v2, t: v3, a: v4 }, ta: { t: v3, a: v4 }, ca: { c: v2, a: v4 },
    e0cta: { e: '', c: v2, t: v3, a: v4 }, c0ta: { c: '', t: v3, a: v4 }, t0a: { t: '', a: v4 },
  };
  const path = CONTEXTS[tag].path;
  if (PASSING_PARENTS.has(path[path.length - 2])) {
    Object.assign(cases, {
      p: { p: v1 }, p0: { p: '' }, pa: { p: v1, a: v4 }, p0a: { p: '', a: v4 }, pt: { p: v1, t: v3 }, pt0: { p: v1, t: '' },
      pc0: { p: v1, c: '' }, pe0: { p: v1, e: '' }, pe: { p: v1, e: v2 }, ptag: { pT: v2 }, ptaga: { pT: v2, a: v4 }, pptag: { p: v1, pT: v2 },
    });
    if (PASSING_PARENTS.has(path[path.length - 3])) {
      Object.assign(cases, { g: { g: v1 }, gp0: { g: v1, p: '' }, gp: { g: v1, p: v2 }, ga: { g: v1, a: v4 }, gt: { g: v1, t: v3 }, g0a: { g: '', a: v4 } });
    }
  }
  return cases;
}
