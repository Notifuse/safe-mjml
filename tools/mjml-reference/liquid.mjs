// Generates mjml/testdata/liquid: templates holding Liquid markup ({{ }} and
// {% %}) where authors and Notifuse's export put it, each written twice:
// as is, and with an inline mj-style whose rules reach the elements holding
// the markup. Each comes with the reference HTML MJML renders for it,
// minified as regenerate.mjs does.
//
// safe-mjml renders templates before their Liquid is evaluated (Notifuse's
// preview and export, and any template it is given raw), so the markup must
// come out as MJML writes it. TestLiquidTemplates checks that safe-mjml
// renders each document as MJML 4.18 does, unless the document is listed in
// testdata/liquid-known-differences.txt with the reason, and that safe-mjml
// keeps every Liquid markup that MJML keeps.
//
// Usage (from this directory, after `npm install`):
//   node liquid.mjs           # (re)write every document, its reference and rejected.json
//   node liquid.mjs --check   # report documents or references that are out of date
//
// rejected.json lists the documents MJML cannot render with --minify (MJML
// throws, or html-minifier refuses its output), with the message.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const mjml2html = require('mjml');
const mjmlVersion = require('mjml/package.json').version;

const here = path.dirname(fileURLToPath(import.meta.url));
const testdata = path.resolve(here, '../../mjml/testdata');
const outDir = path.join(testdata, 'liquid');

// A document: head content, then sections in the body.
const doc = ({ head = '', body, bodyAttrs = '' }) =>
  `<mjml>\n  <mj-head>\n    <mj-title>Order {{ order.number }}</mj-title>\n${head}  </mj-head>\n  <mj-body${bodyAttrs}>\n${body}  </mj-body>\n</mjml>\n`;
const section = (columnContent, attrs = '') =>
  `    <mj-section${attrs}>\n      <mj-column>\n${columnContent}      </mj-column>\n    </mj-section>\n`;
const text = (content, attrs = '') => `        <mj-text${attrs}>\n          ${content}\n        </mj-text>\n`;
const button = (content, attrs = ' href="https://example.com/shop"') => `        <mj-button${attrs}>${content}</mj-button>\n`;
const image = (attrs) => `        <mj-image${attrs} />\n`;
const plain = section(text('<p class="liq">Thank you for your order.</p>'));

// Each case: a name and its document. The cases cover the places the
// markup goes: attributes of mj-* tags (with filters and quotes), the
// content of ending tags, HTML attributes and style attributes in that
// content, blocks around components (the shape of Notifuse's display
// conditions), the head (title, preview, mj-attributes, mj-class, mj-font,
// mj-style), whitespace control, {% raw %} and {% comment %}.
const CASES = {
  // Attributes of mj-* tags.
  'button-href-variable': doc({ body: section(button('Track my order', ' href="{{ order.tracking_url }}"')) }),
  'button-href-filter-single-quotes': doc({ body: section(button('Shop', ` href="{{ shop_url | append: '?utm_source=email' }}"`)) }),
  'button-href-filter-double-quotes': doc({ body: section(button('Shop', ` href='{{ shop_url | append: "?utm_source=email" }}'`)) }),
  'button-href-if': doc({ body: section(button('Open', ' href="{% if contact.vip %}https://example.com/vip{% else %}https://example.com{% endif %}"')) }),
  'button-content-filter': doc({ body: section(button('{{ "buy now" | upcase }}')) }),
  'button-color-variable': doc({ body: section(button('Shop', ' href="https://example.com" background-color="{{ brand.color }}" color="{{ brand.text_color }}"')) }),
  'image-src-alt': doc({ body: section(image(' src="{{ product.image_url }}" alt="{{ product.name | escape }}" href="{{ product.url }}"')) }),
  'section-background-variable': doc({ body: section(text('<p>Hello</p>'), ' background-color="{{ brand.background }}" background-url="{{ brand.banner_url }}"') }),
  'text-color-and-padding-variables': doc({ body: section(text('<p>Hello</p>', ' color="{{ brand.color }}" padding="{{ layout.padding }}" font-size="{{ layout.font_size }}"')) }),
  'text-font-family-variable': doc({ body: section(text('<p>Hello</p>', ' font-family="{{ brand.font }}"')) }),
  'css-class-variable': doc({ body: section(text('<p>Hello</p>', ' css-class="{{ theme }}-text"')) }),
  'social-element-href': doc({ body: section('        <mj-social>\n          <mj-social-element name="facebook" href="{{ social.facebook }}">Facebook</mj-social-element>\n          <mj-social-element name="x" href="https://x.com/{{ social.x_handle }}">{{ social.x_label }}</mj-social-element>\n        </mj-social>\n') }),
  'navbar-link-href': doc({ body: section('        <mj-navbar>\n          <mj-navbar-link href="{{ links.home }}">Home</mj-navbar-link>\n          <mj-navbar-link href="/account?id={{ contact.id }}">{{ "Account" | t }}</mj-navbar-link>\n        </mj-navbar>\n') }),
  'divider-color-variable': doc({ body: section(`        <mj-divider border-color="{{ brand.color }}" border-width="{{ layout.rule }}" />\n${text('<p>Hello</p>')}`) }),
  'body-background-variable': doc({ bodyAttrs: ' background-color="{{ brand.page_color }}"', body: plain }),

  // Content of ending tags.
  'text-variables': doc({ body: section(text('<p class="liq">Hello {{ contact.first_name | default: "there" }}, your order {{ order.number }} ships on {{ order.date | date: "%B %-d" }}.</p>')) }),
  'text-if-else': doc({ body: section(text('{% if contact.vip %}<p class="liq"><b>Thank you for being a VIP.</b></p>{% else %}<p class="liq">Thank you.</p>{% endif %}')) }),
  'text-for-loop': doc({ body: section(text('<ul>{% for item in order.items %}<li class="liq">{{ item.quantity }} × {{ item.name }}: {{ item.price | money }}</li>{% endfor %}</ul>')) }),
  'text-comparisons': doc({ body: section(text('{% if order.total > 100 %}<p>Free shipping</p>{% endif %}{% if order.total < 20 %}<p>Add more</p>{% endif %}{% if a >= 1 and b <= 2 %}<p>Both</p>{% endif %}')) }),
  'text-link-href-and-label': doc({ body: section(text('<p>Read <a class="liq" href="{{ article.url }}?ref={{ contact.id | url_encode }}" title="{{ article.title }}">{{ article.title }}</a>.</p>')) }),
  'text-span-style-variable': doc({ body: section(text('<p>Total: <span class="liq" style="color: {{ brand.color }}; font-weight: bold">{{ order.total }}</span></p>')) }),
  'text-style-if': doc({ body: section(text('<p class="liq" style="{% if contact.vip %}color: #b8860b;{% endif %} margin: 0">Hello</p>')) }),
  'text-style-only-variable': doc({ body: section(text('<p class="liq" style="{{ styles.paragraph }}">Hello</p>')) }),
  'text-tag-spanning-elements': doc({ body: section(text('<p>{% if order.gift %}<b>A gift</b> for {{ order.recipient }}</p><p>{% endif %}Thank you</p>')) }),
  'text-whitespace-control': doc({ body: section(text('<p class="liq">{%- if contact.first_name -%} Hello {{- contact.first_name -}}! {%- endif -%}</p>')) }),
  'text-raw-block': doc({ body: section(text('<p>{% raw %}Write {{ name }} to insert a name{% endraw %}</p>')) }),
  'text-comment-block': doc({ body: section(text('{% comment %}Shown to VIPs only{% endcomment %}<p>Hello</p>')) }),
  'text-assign-and-capture': doc({ body: section(text('{% assign greeting = "Hello" %}{% capture name %}{{ contact.first_name | capitalize }}{% endcapture %}<p class="liq">{{ greeting }} {{ name }}</p>')) }),
  'text-ampersand-in-filter': doc({ body: section(text('<p class="liq">{{ company | replace: "&", "and" }} &amp; {{ partner }}</p>')) }),
  'table-for-loop': doc({ body: section('        <mj-table>\n          <tr><th>Item</th><th>Price</th></tr>\n          {% for item in order.items %}<tr><td class="liq" style="padding: 0 15px 0 0">{{ item.name }}</td><td style="color: {{ brand.color }}">{{ item.price }}</td></tr>{% endfor %}\n        </mj-table>\n') }),
  'raw-if-block': doc({ body: `    {% if contact.vip %}\n    <mj-raw><div class="liq" style="padding: 10px">VIP corner</div></mj-raw>\n    {% endif %}\n${plain}` }),
  'raw-whole-tag-markup': doc({ body: `    <mj-raw>{% unless contact.unsubscribed %}<a class="liq" href="{{ unsubscribe_url }}">Unsubscribe</a>{% endunless %}</mj-raw>\n${plain}` }),
  'accordion-variables': doc({ body: section('        <mj-accordion>\n          <mj-accordion-element>\n            <mj-accordion-title>{{ faq.question }}</mj-accordion-title>\n            <mj-accordion-text><p class="liq">{{ faq.answer }}</p></mj-accordion-text>\n          </mj-accordion-element>\n        </mj-accordion>\n') }),

  // Blocks around components.
  'if-around-section': doc({ body: `${plain}    {% if order.gift %}\n${section(text('<p>A gift message</p>'))}    {% endif %}\n` }),
  'if-around-column': doc({ body: `    <mj-section>\n      {% if contact.vip %}\n      <mj-column>\n${text('<p>VIP</p>')}      </mj-column>\n      {% endif %}\n      <mj-column>\n${text('<p>Everyone</p>')}      </mj-column>\n    </mj-section>\n` }),
  'if-around-text': doc({ body: section(`        {% if contact.first_name %}\n${text('<p>Hello {{ contact.first_name }}</p>')}        {% else %}\n${text('<p>Hello</p>')}        {% endif %}\n`) }),
  'for-around-section': doc({ body: `    {% for product in products %}\n${section(text('<p class="liq">{{ product.name }}</p>') + button('Buy', ' href="{{ product.url }}"'))}    {% endfor %}\n` }),
  'if-around-wrapper': doc({ body: `    {% if banner %}\n    <mj-wrapper background-color="#f0f0f0">\n${plain}    </mj-wrapper>\n    {% endif %}\n` }),
  // The display condition Notifuse's export writes around a block: one line
  // of tags, without "<", around the block's MJML.
  'notifuse-display-condition': doc({ body: `    {% assign dc_flag_b1 = true %}{% assign dc_ok_b1 = false %}{% assign dc_vs_b1 = contact.orders | append: "" | strip %}{% if dc_vs_b1 == "-" or dc_vs_b1 == "+" %}{% assign dc_b_b1 = dc_vs_b1 | slice: 1, 1000000 %}{% endif %}{% if dc_n_b1 > 5 %}{% assign dc_ok_b1 = true %}{% endif %}{% unless dc_ok_b1 %}{% assign dc_flag_b1 = false %}{% endunless %}{% if dc_flag_b1 %}\n${plain}    {% endif %}\n` }),

  // Head.
  'preview-variable': doc({ head: '    <mj-preview>Your order {{ order.number }} is on its way</mj-preview>\n', body: plain }),
  'attributes-variables': doc({ head: '    <mj-attributes>\n      <mj-all font-family="{{ brand.font }}" />\n      <mj-text color="{{ brand.color }}" />\n      <mj-button background-color="{{ brand.color }}" />\n    </mj-attributes>\n', body: section(text('<p>Hello</p>') + button('Shop')) }),
  'mj-class-variables': doc({ head: '    <mj-attributes>\n      <mj-class name="brand" color="{{ brand.color }}" background-color="{{ brand.background }}" />\n    </mj-attributes>\n', body: section(text('<p>Hello</p>', ' mj-class="brand"') + button('Shop', ' mj-class="brand" href="https://example.com"')) }),
  'font-href-variable': doc({ head: '    <mj-font name="Brand" href="{{ brand.font_url }}" />\n', body: section(text('<p>Hello</p>', ' font-family="Brand, Arial"')) }),
  'style-variable-in-css': doc({ head: '    <mj-style>.liq { color: {{ brand.color }}; }</mj-style>\n', body: plain }),
  'style-if-in-css': doc({ head: '    <mj-style>{% if dark %}.liq { color: #ffffff; }{% endif %}</mj-style>\n', body: plain }),
};

// The inline variant adds an inline mj-style whose rules reach the
// elements holding the markup (class liq, links, cells, spans, images, the
// mj-text and mj-button cells), so juice rewrites their style attributes.
const INLINE = '    <mj-style inline="inline">.liq { font-weight: bold; } a { color: #0b57d0; } span, li { line-height: 20px; } img { border-radius: 4px; }</mj-style>\n';
const withInline = (source) => source.replace('  </mj-head>', `${INLINE}  </mj-head>`);

function documents() {
  const out = new Map();
  for (const [name, source] of Object.entries(CASES)) {
    out.set(name, source);
    out.set(`${name}--inline`, withInline(source));
  }
  return out;
}

const check = process.argv.includes('--check');
const docs = documents();
fs.mkdirSync(outDir, { recursive: true });
const stale = [];
const rejected = {};
const current = (f) => (fs.existsSync(f) ? fs.readFileSync(f, 'utf8') : null);
// Carousel and navbar IDs are random in each MJML run: a reference that
// differs from the new rendering only by them is kept as it is.
const withoutIds = (h) => h && h.replace(/[0-9a-f]{16}/g, 'ID');
for (const [name, source] of docs) {
  const mjmlFile = path.join(outDir, `${name}.mjml`);
  const htmlFile = path.join(outDir, `${name}.html`);
  let html = null;
  try {
    html = mjml2html(source, { minify: true, validationLevel: 'soft' }).html;
  } catch (err) {
    rejected[name] = String(err.message).split('\n')[0];
  }
  if (html !== null && current(mjmlFile) === source && withoutIds(current(htmlFile)) === withoutIds(html)) html = current(htmlFile);
  if (current(mjmlFile) !== source || current(htmlFile) !== html) {
    stale.push(name);
    if (!check) {
      fs.writeFileSync(mjmlFile, source);
      if (html === null) {
        if (fs.existsSync(htmlFile)) fs.renameSync(htmlFile, `${htmlFile}.stale`);
      } else {
        fs.writeFileSync(htmlFile, html);
      }
    }
  }
}
const rejectedFile = path.join(outDir, 'rejected.json');
const sorted = Object.fromEntries(Object.keys(rejected).sort().map((k) => [k, rejected[k]]));
const rejectedText = `${JSON.stringify(sorted, null, 2)}\n`;
if (current(rejectedFile) !== rejectedText) {
  stale.push('rejected.json');
  if (!check) fs.writeFileSync(rejectedFile, rejectedText);
}
const extra = fs.readdirSync(outDir).filter((f) => f.endsWith('.mjml') && !docs.has(f.slice(0, -5)));
for (const f of extra) stale.push(`${f.slice(0, -5)} (not generated any more: move it out)`);
console.log(`MJML ${mjmlVersion}: ${docs.size} documents with Liquid, ${Object.keys(rejected).length} rejected by MJML, ${stale.length} ${check ? 'out of date' : 'written'}`);
for (const s of stale.slice(0, 20)) console.log(`  ${s}`);
process.exitCode = check && stale.length ? 1 : 0;
