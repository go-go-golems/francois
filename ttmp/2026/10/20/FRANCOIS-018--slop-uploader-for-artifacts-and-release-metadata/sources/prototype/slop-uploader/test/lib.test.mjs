import { test } from 'node:test';
import assert from 'node:assert/strict';
import { slugify, validID, nextVersion, buildManifest, manifestKeys, htmlTitle, parseTags } from '../public/lib.js';

test('slugify makes valid artifact IDs', () => {
  assert.equal(slugify('Lattice Rift'), 'lattice-rift');
  assert.equal(slugify('  Café — Déjà Vu!! '), 'cafe-deja-vu');
  assert.equal(slugify('***'), '');
  const long = slugify('a '.repeat(80));
  assert.ok(long.length <= 64 && validID(long), long);
});

test('nextVersion bumps the highest version', () => {
  assert.equal(nextVersion([]), '0.1.0');
  assert.equal(nextVersion(['0.1.0', '0.1.2', '0.1.1']), '0.1.3');
  assert.equal(nextVersion(['0.9.0', '0.10.0']), '0.10.1');
  assert.equal(nextVersion(['1.0.0-beta']), '1.0.1');
});

test('manifest round-trips through the checkboxes', () => {
  assert.deepEqual(buildManifest(['sample']), { sample: {} });
  assert.deepEqual(buildManifest(['images', 'db', 'downloads']), { sample: { images: true }, db: {}, downloads: true });
  assert.deepEqual(buildManifest(['user']), { user: { scopes: ['profile'] } });
  assert.deepEqual(manifestKeys({ sample: { images: true }, room: {} }).sort(), ['images', 'room', 'sample']);
  assert.deepEqual(manifestKeys(null), ['sample']);
});

test('htmlTitle reads the document title', () => {
  assert.equal(htmlTitle('<html><head><title>\n  Polis &amp; Co </title>'), 'Polis & Co');
  assert.equal(htmlTitle('<html></html>'), '');
});

test('parseTags trims, lowercases and dedupes', () => {
  assert.deepEqual(parseTags(' Graphics, webgl ,,graphics'), ['graphics', 'webgl']);
});
