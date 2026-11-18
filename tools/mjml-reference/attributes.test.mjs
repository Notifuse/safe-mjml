// Checks that internal/components/allowed-css-attributes.json, the
// attributes and types safe-mjml validates, is MJML's (node --test).
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
require('mjml');
const { components } = require('mjml-core');
const allowed = JSON.parse(fs.readFileSync(new URL('../../internal/components/allowed-css-attributes.json', import.meta.url), 'utf8'));

test('allowed-css-attributes.json has the allowedAttributes of every MJML component', () => {
  const differences = [];
  for (const name of new Set([...Object.keys(components), ...Object.keys(allowed)])) {
    const want = components[name]?.allowedAttributes ?? {};
    const got = allowed[name] ?? {};
    for (const attr of new Set([...Object.keys(want), ...Object.keys(got)])) {
      if (want[attr] !== got[attr]) differences.push(`${name} ${attr}: MJML ${want[attr]}, safe-mjml ${got[attr]}`);
    }
  }
  assert.deepEqual(differences, []);
});
