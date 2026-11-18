// Checks the reference pipeline of render.mjs (node --test): the HTML is
// MJML 5's, compacted as the comparison with safe-mjml expects.
import test from 'node:test';
import assert from 'node:assert/strict';
import { renderReference, mjmlVersion } from './render.mjs';

const doc = `<mjml>
  <mj-body css-class="b">
    <mj-section>
      <mj-column>
        <mj-text css-class="">Hello   <b>world</b></mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`;

test('renders with the MJML of package.json', () => {
  assert.equal(mjmlVersion, '5.4.1');
});

test('mj-body writes the body, as in MJML 5', async () => {
  const { html } = await renderReference(doc);
  assert.match(html, /<body class="b" style="word-spacing:normal;"><div aria-roledescription="email" role="article" lang="und" dir="auto" style="word-spacing:normal;">/);
});

test('the minifier collapses whitespace and drops empty attributes', async () => {
  const { html } = await renderReference(doc);
  // The content of <style> is left as written (minifyCSS: false).
  const body = html.slice(html.indexOf('<body'));
  assert.doesNotMatch(body, />\s+</);
  // Outlook conditional comments are left as MJML writes them, class="" included.
  assert.doesNotMatch(body.replace(/<!--[\s\S]*?-->/g, ''), /\s(?:style|class)=""/);
  assert.match(html, /Hello <b>world<\/b>/);
});

test('the CSS is left as MJML writes it', async () => {
  const { html } = await renderReference(doc);
  // cssnano, which MJML 5's own minify runs, would write "Ubuntu,Helvetica".
  assert.match(html, /font-family:Ubuntu, Helvetica, Arial, sans-serif;/);
});

test('validation errors come with the HTML', async () => {
  const { errors } = await renderReference('<mjml><mj-body><mj-section><mj-column><mj-text bogus="1">x</mj-text></mj-column></mj-section></mj-body></mjml>');
  assert.equal(errors.length, 1);
  assert.match(errors[0].message, /bogus/);
});
