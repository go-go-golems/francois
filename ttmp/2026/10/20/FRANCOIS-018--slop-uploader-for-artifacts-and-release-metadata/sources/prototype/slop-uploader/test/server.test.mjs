import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { createServer, forwardable } from '../server.mjs';

let upstream, uploader, base, port;
const seen = [];

before(async () => {
  upstream = http.createServer((req, res) => {
    let body = '';
    req.on('data', (c) => (body += c));
    req.on('end', () => {
      seen.push({ method: req.method, url: req.url, auth: req.headers.authorization, type: req.headers['content-type'], body });
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ ok: true }));
    });
  });
  await new Promise((r) => upstream.listen(0, '127.0.0.1', r));
  uploader = createServer({ upstream: `http://127.0.0.1:${upstream.address().port}/`, token: 'frt_test', secret: 's3cret' });
  await new Promise((r) => uploader.listen(0, '127.0.0.1', r));
  port = uploader.address().port;
  base = `http://127.0.0.1:${port}`;
});

after(() => {
  uploader.close();
  upstream.close();
});

test('serves the page with the secret but not the token', async () => {
  const html = await (await fetch(base + '/')).text();
  assert.match(html, /s3cret/);
  assert.doesNotMatch(html, /frt_test/);
});

test('forwards studio calls with the token and body', async () => {
  const res = await fetch(base + '/api/studio/artifacts/x', {
    method: 'PATCH',
    headers: { 'x-uploader-secret': 's3cret', 'content-type': 'application/json', origin: base },
    body: JSON.stringify({ title: 'T' }),
  });
  assert.equal(res.status, 200);
  const last = seen.at(-1);
  assert.equal(last.method, 'PATCH');
  assert.equal(last.url, '/api/studio/artifacts/x');
  assert.equal(last.auth, 'Bearer frt_test');
  assert.equal(last.body, '{"title":"T"}');
});

test('forwards multipart uploads', async () => {
  const form = new FormData();
  form.set('artifactId', 'demo');
  form.set('html', new Blob(['<!doctype html><title>x</title>'], { type: 'text/html' }), 'demo.html');
  const res = await fetch(base + '/api/studio/releases/check', { method: 'POST', headers: { 'x-uploader-secret': 's3cret' }, body: form });
  assert.equal(res.status, 200);
  const last = seen.at(-1);
  assert.match(last.type, /^multipart\/form-data; boundary=/);
  assert.match(last.body, /<!doctype html>/);
});

test('refuses calls without the secret', async () => {
  const before = seen.length;
  const res = await fetch(base + '/api/me');
  assert.equal(res.status, 403);
  assert.equal(seen.length, before);
});

test('refuses other origins and hosts', async () => {
  const before = seen.length;
  const cross = await fetch(base + '/api/me', { headers: { 'x-uploader-secret': 's3cret', origin: 'https://evil.example' } });
  assert.equal(cross.status, 403);
  const rebound = await new Promise((resolve) => {
    http.get({ host: '127.0.0.1', port, path: '/', headers: { host: `evil.example:${port}` } }, resolve);
  });
  assert.equal(rebound.statusCode, 403);
  assert.equal(seen.length, before);
});

test('forwards only the studio API and /api/me', () => {
  assert.ok(forwardable('/api/me'));
  assert.ok(forwardable('/api/studio/releases'));
  assert.ok(!forwardable('/api/admin/users'));
  assert.ok(!forwardable('/api/account'));
});
