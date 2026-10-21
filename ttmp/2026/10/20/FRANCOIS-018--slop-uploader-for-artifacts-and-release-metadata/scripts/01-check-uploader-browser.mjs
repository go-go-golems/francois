// Real built UI, deterministic API fixtures. Go tests verify real server contracts.
import { createRequire } from 'node:module';
import { readFile, mkdir, writeFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
const require = createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright/package.json');
const { chromium } = require('playwright');
const out = new URL(process.argv[2] ?? '../sources/implementation/', import.meta.url);
await mkdir(out, { recursive: true });
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } });
const errors = [], writes = [];
let failAttachment = false;
let release;
const artifact = { id: 'lattice-notebook', title: 'Lattice Notebook', description: 'A small workspace for connected ideas.', tags: ['tools', 'notes'], page: { body: '', gallery: [] }, visibility: 'listed' };
page.on('pageerror', e => errors.push(e.message));
await page.route('http://fixture.test/**', async route => {
  const req = route.request(), path = new URL(req.url()).pathname, method = req.method();
  if (path.startsWith('/static/')) return route.fulfill({ body: await readFile('pkg/catalog/web' + path), contentType: path.endsWith('.css') ? 'text/css' : 'application/javascript' });
  let data;
  if (method !== 'GET') writes.push({ method, path });
  if (path === '/api/session') data = { authenticated: true, user: { id: 'usr_admin', role: 'admin', name: 'Ramon Marc', email: 'ramon@example.test' }, supporter: true, csrfToken: 'fixture', brand: 'slop.boutique', plan: { source: 'staff', paid: true, instances: 10 } };
  else if (path === '/api/studio/me') data = { user: { id: 'usr_admin', role: 'admin', name: 'Ramon Marc' }, creator: null, limits: { maxHTMLBytes: 16777216, maxMediaBytes: 26214400, maxAttachmentBytes: 104857600, maxGalleryItems: 24, maxAttachments: 20 } };
  else if (path === '/api/studio/artifacts') data = [{ id: 'existing', title: 'Existing experiment' }];
  else if (path === '/api/studio/artifacts/existing') data = { artifact: { ...artifact, id: 'existing', title: 'Existing experiment' }, creators: [], releases: [{ id: 'rel_existing', version: '1.0.0', state: 'published', publishedAt: '2026-10-20T00:00:00Z', manifest: { sample: { images: true }, slop_boutique: { artifacts: true } }, freeSample: true }] };
  else if (path === '/api/studio/media') data = method === 'GET' ? [] : { id: 'med_cover', mediaType: 'image/png' };
  else if (path === '/api/studio/markdown') data = { html: '<h2>A notebook for connected ideas</h2><p>Collect notes, explore relationships, and keep the useful parts.</p>' };
  else if (path === '/api/studio/releases/check') data = { blocking: false, checks: [{ message: 'Authored HTML validated', level: 'info' }] };
  else if (path === '/api/studio/releases' && method === 'POST') { release = { id: 'rel_fixture', version: '0.1.0', state: 'draft', checks: [] }; data = release; }
  else if (path.endsWith('/attachments') && failAttachment) return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'unavailable', message: 'Attachment storage temporarily unavailable' } }) });
  else if (path.endsWith('/releases/rel_fixture/publish')) { release.state = 'published'; data = release; }
  else if (path === '/api/studio/artifacts/lattice-notebook' && method === 'GET') data = { artifact, creators: [], releases: release ? [release] : [] };
  else if (path.startsWith('/api/studio/')) data = {};
  else if (path.startsWith('/fonts/')) return route.fulfill({ status: 404, body: '' });
  else if (path.startsWith('/media/')) return route.fulfill({ status: 404, body: '' });
  else return route.fulfill({ contentType: 'text/html', body: '<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/static/catalog.css"><div id="root"></div><script src="/static/catalog.js"></script>' });
  return route.fulfill({ contentType: 'application/json', body: JSON.stringify(data) });
});
async function fill() {
  await page.goto('http://fixture.test/studio/upload');
  await page.locator('[data-field=html]').setInputFiles({ name: 'lattice-notebook.html', mimeType: 'text/html', buffer: Buffer.from('<!doctype html><html><head><title>Lattice Notebook</title></head><body><h1>Notebook</h1></body></html>') });
  await page.waitForFunction(() => document.querySelector('[data-field=id]').value === 'lattice-notebook');
  await page.getByLabel('Tags, separated by commas').fill('tools, notes, experiments');
  await page.getByLabel('Short description').fill('A small workspace for connected ideas. Collect notes, explore relationships, and keep the useful parts.');
  await page.getByLabel('Release notes').fill('First public release with a local notebook and model-assisted summaries.');
  await page.getByLabel('Workspace database').check();
  await page.getByLabel('Public download', { exact: true }).check();
  await page.getByLabel('Upload cover', { exact: true }).setInputFiles('ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/various/showcase/lattice-deck-cover.png');
  await page.getByLabel('The write-up (Markdown)').fill('## A notebook for connected ideas\n\nCollect notes, explore relationships, and keep the useful parts.');
}
try {
  await fill();
  await page.getByRole('heading', { name: 'A notebook for connected ideas', exact: true }).waitFor();
  await page.evaluate(() => window.scrollTo(0,0));
  await page.screenshot({ path: new URL('uploader-overview-1440.png', out).pathname });
  await page.locator('.upload-panel').nth(3).screenshot({ path: new URL('uploader-storefront-1440.png', out).pathname });
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 1100 });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `overflow at ${width}`);
    await page.evaluate(() => window.scrollTo(0,0)); await page.waitForTimeout(400);
    await page.screenshot({ path: new URL(`uploader-filled-${width}.png`, out).pathname, fullPage: true });
  }
  await page.setViewportSize({ width: 1440, height: 1100 });
  await page.getByRole('button', { name: 'Publish artifact' }).click();
  await page.getByText('Published', { exact: true }).waitFor();
  assert.equal(release.state, 'published');
  assert.equal(writes.filter(w => w.path === '/api/studio/releases').length, 1);
  await page.evaluate(() => window.scrollTo(0,0));
  await page.screenshot({ path: new URL('uploader-published-1440.png', out).pathname, fullPage: true });
  page.on('dialog', d => d.accept());
  failAttachment = true; await fill();
  await page.getByLabel('Attachments', { exact: true }).setInputFiles({ name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('Source notes') });
  await page.getByText('notes.txt', { exact: false }).waitFor();
  await page.getByRole('button', { name: 'Publish artifact' }).click();
  await page.getByText('Upload stopped', { exact: true }).waitFor();
  await page.getByText('Release 0.1.0: draft.', { exact: true }).waitFor();
  assert(await page.getByRole('button', { name: 'Publish artifact' }).isDisabled());
  await page.evaluate(() => window.scrollTo(0,0));
  await page.screenshot({ path: new URL('uploader-recovery-1440.png', out).pathname, fullPage: true });
  await page.getByRole('combobox').selectOption('existing');
  await page.getByText('Title, description and tags update the live catalog', { exact: false }).waitFor();
  assert.equal(await page.locator('[data-field=html]').inputValue(), '');
  assert.equal(await page.getByLabel('Version', { exact: true }).inputValue(), '1.0.1');
  assert.equal(await page.getByLabel('Title', { exact: true }).inputValue(), 'Existing experiment');
  await page.getByText('Manifest JSON', { exact: true }).click();
  assert.match(await page.getByLabel('Full capability manifest').inputValue(), /slop_boutique/);
  await page.locator('[data-field=html]').setInputFiles({ name: 'new-version.html', mimeType: 'text/html', buffer: Buffer.from('<!doctype html><title>Replacement file title</title>') });
  assert.equal(await page.getByLabel('Title', { exact: true }).inputValue(), 'Existing experiment');
  failAttachment = false;
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await page.getByText('Draft saved', { exact: true }).waitFor();
  assert.deepEqual(errors, []);
  const receipt = { mockedAPI: true, realBuiltUI: true, viewports: [1440, 390], actions: ['HTML title and ID autofill', 'edit capabilities and details', 'publish', 'attachment failure', 'reconcile draft', 'block replay', 'switch artifacts clears staged files', 'prefill next version and full manifest', 'save draft'], pageErrors: errors, writes };
  await writeFile(new URL('browser.json', out), JSON.stringify(receipt, null, 2) + '\n');
  console.log(JSON.stringify(receipt));
} catch (error) { console.log(JSON.stringify({ errors, writes, text: await page.locator('body').innerText() })); throw error; } finally { await browser.close(); }
