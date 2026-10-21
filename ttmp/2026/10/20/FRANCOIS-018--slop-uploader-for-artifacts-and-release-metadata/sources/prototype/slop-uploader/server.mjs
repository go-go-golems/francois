#!/usr/bin/env node
// Local uploader for slop.boutique. Serves the form in public/ and forwards
// its calls to the catalog's studio API, adding the personal API token. The
// token never reaches the browser.
import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { randomBytes } from 'node:crypto';
import { homedir } from 'node:os';
import { join, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Readable } from 'node:stream';
import { execFile } from 'node:child_process';

const publicDir = fileURLToPath(new URL('./public/', import.meta.url));
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8' };
const assets = new Set(['/app.js', '/lib.js', '/style.css']);

export async function loadToken(env = process.env) {
  if (env.FRANCOIS_TOKEN) return env.FRANCOIS_TOKEN.trim();
  const file = env.FRANCOIS_TOKEN_FILE || join(homedir(), '.config/francois/token');
  try {
    return (await readFile(file, 'utf8')).trim();
  } catch {
    throw new Error(`No API token: put it in ${file} or set FRANCOIS_TOKEN`);
  }
}

// Only the studio API and the identity check are forwarded.
export function forwardable(path) {
  return path === '/api/me' || path.startsWith('/api/studio/');
}

export function createServer({ upstream, token, secret = randomBytes(24).toString('hex') }) {
  upstream = upstream.replace(/\/+$/, '');
  const server = http.createServer(async (req, res) => {
    try {
      await handle(req, res);
    } catch (err) {
      send(res, 502, { error: { code: 'upstream', message: `Could not reach ${upstream}: ${err.message}` } });
    }
  });

  // A page on another site must not be able to drive this server with your
  // token: requests have to name this server as their host (no DNS
  // rebinding), come from its own origin, and carry the secret embedded in
  // the page.
  function trusted(req) {
    const port = server.address().port;
    const hosts = [`127.0.0.1:${port}`, `localhost:${port}`];
    if (!hosts.includes(req.headers.host)) return false;
    const origin = req.headers.origin;
    if (origin && !hosts.some((h) => origin === `http://${h}`)) return false;
    return true;
  }

  async function handle(req, res) {
    const url = new URL(req.url, 'http://local');
    if (!trusted(req)) return send(res, 403, { error: { code: 'forbidden', message: 'Untrusted host or origin' } });

    if (req.method === 'GET' && url.pathname === '/') {
      const page = (await readFile(join(publicDir, 'index.html'), 'utf8'))
        .replaceAll('{{SECRET}}', secret)
        .replaceAll('{{SERVER}}', upstream);
      return send(res, 200, page, types['.html']);
    }
    if (req.method === 'GET' && assets.has(url.pathname)) {
      return send(res, 200, await readFile(join(publicDir, url.pathname)), types[extname(url.pathname)]);
    }
    if (!url.pathname.startsWith('/api/')) return send(res, 404, { error: { code: 'not_found', message: 'Not found' } });
    if (req.headers['x-uploader-secret'] !== secret) {
      return send(res, 403, { error: { code: 'forbidden', message: 'Missing uploader secret; reload the page' } });
    }
    if (!forwardable(url.pathname)) return send(res, 404, { error: { code: 'not_found', message: 'Not forwarded' } });

    const headers = { authorization: `Bearer ${token}`, accept: 'application/json' };
    if (req.headers['content-type']) headers['content-type'] = req.headers['content-type'];
    const hasBody = !['GET', 'HEAD'].includes(req.method);
    const up = await fetch(upstream + url.pathname + url.search, {
      method: req.method,
      headers,
      body: hasBody ? req : undefined,
      duplex: hasBody ? 'half' : undefined,
    });
    res.writeHead(up.status, { 'content-type': up.headers.get('content-type') || 'application/octet-stream' });
    if (up.body) Readable.fromWeb(up.body).pipe(res);
    else res.end();
  }

  return server;
}

function send(res, status, body, type) {
  const isJSON = !type;
  res.writeHead(status, { 'content-type': type || 'application/json', 'cache-control': 'no-store' });
  res.end(isJSON ? JSON.stringify(body) : body);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const upstream = process.env.SLOP_SERVER || 'https://slop.boutique';
  const port = Number(process.env.PORT || 4747);
  let token;
  try {
    token = await loadToken();
  } catch (err) {
    console.error(err.message);
    process.exit(1);
  }
  const server = createServer({ upstream, token });
  server.listen(port, '127.0.0.1', () => {
    const url = `http://127.0.0.1:${server.address().port}`;
    console.log(`slop uploader → ${url}  (uploading to ${upstream}; Ctrl-C to stop)`);
    if (process.platform === 'darwin' && !process.env.NO_OPEN) execFile('open', [url]);
  });
}
