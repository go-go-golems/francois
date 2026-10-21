import {
  CAPABILITIES, buildManifest, manifestKeys, slugify, validID, validVersion,
  nextVersion, htmlTitle, parseTags, formatBytes,
} from './lib.js';

const SECRET = document.querySelector('meta[name=uploader-secret]').content;
const SERVER = document.querySelector('meta[name=uploader-server]').content;
const $ = (id) => document.getElementById(id);
const enc = encodeURIComponent;

const state = {
  current: null, // the selected artifact's studio detail; null for a new one
  html: null, // File
  cover: null, // {id} already on the site, or {file} to upload
  gallery: [],
  attachments: [],
  original: null, // page as loaded, to skip saving it unchanged
  hasPageDraft: false,
  idTouched: false,
  limits: { maxMediaBytes: 25 << 20, maxAttachmentBytes: 100 << 20 },
  mediaTypes: new Map(), // media id → MIME type, for thumbnails
  busy: false,
};

// ---------------------------------------------------------------- API

async function api(path, { method = 'GET', json, form } = {}) {
  const headers = { 'x-uploader-secret': SECRET };
  let body = form;
  if (json !== undefined) {
    headers['content-type'] = 'application/json';
    body = JSON.stringify(json);
  }
  const res = await fetch('/api' + path, { method, headers, body });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { error: { message: text.slice(0, 200) } };
  }
  if (!res.ok) {
    const err = new Error(data?.error?.message || `${res.status} ${res.statusText}`);
    err.data = data;
    throw err;
  }
  return data;
}

function formData(fields) {
  const fd = new FormData();
  for (const [k, v] of Object.entries(fields)) {
    if (v instanceof File) fd.set(k, v, k === 'html' ? `${fields.artifactId}.html` : v.name);
    else fd.set(k, String(v));
  }
  return fd;
}

// ---------------------------------------------------------------- setup

async function init() {
  $('caps').innerHTML = CAPABILITIES.map((c) =>
    `<label class="check"><input type="checkbox" value="${c.key}"><span>${c.label}<small>${c.hint}</small></span></label>`).join('');
  setCaps(['sample']);
  $('version').value = '0.1.0';

  dropZone($('html-drop'), (files) => pickHTML(files[0]));
  dropZone($('cover-drop'), (files) => { state.cover = { file: files[0] }; renderMedia(); });
  dropZone($('gallery-drop'), (files) => { state.gallery.push(...files.map((file) => ({ file }))); renderMedia(); });
  dropZone($('attach-drop'), (files) => { state.attachments.push(...files); renderAttachments(); });

  $('artifact').addEventListener('change', () => selectArtifact($('artifact').value));
  $('title').addEventListener('input', () => {
    if (!state.current && !state.idTouched) $('id').value = slugify($('title').value);
  });
  $('id').addEventListener('input', () => { state.idTouched = true; });
  $('preview-toggle').addEventListener('click', togglePreview);
  $('check').addEventListener('click', () => run('check'));
  $('draft').addEventListener('click', () => run('draft'));
  $('publish').addEventListener('click', () => run('publish'));

  try {
    const [me, studio] = await Promise.all([api('/me'), api('/studio/me')]);
    state.limits = studio.limits || state.limits;
    let who = `${me.user.name} · ${me.user.role} · uploading to ${new URL(SERVER).host}`;
    if (me.user.role === 'creator' && !me.creator) who += ' · no creator profile yet: ask an admin to link one before creating new artifacts';
    $('who').textContent = who;
  } catch (e) {
    $('who').textContent = `Could not sign in to ${SERVER}: ${e.message}`;
    $('who').style.color = 'var(--bad)';
    return;
  }
  await Promise.all([loadArtifacts(), loadMediaTypes()]);
}

async function loadArtifacts(select) {
  const list = await api('/studio/artifacts');
  list.sort((a, b) => a.title.localeCompare(b.title));
  $('artifact').innerHTML = '<option value="">New artifact</option>' + list.map((a) =>
    `<option value="${esc(a.id)}">${esc(a.title)} (${esc(a.id)}${a.latestVersion ? ' · ' + esc(a.latestVersion) : ''})</option>`).join('');
  if (select) {
    $('artifact').value = select;
    await selectArtifact(select);
  }
}

async function loadMediaTypes() {
  try {
    for (const m of await api('/studio/media')) state.mediaTypes.set(m.id, m.mediaType);
  } catch { /* thumbnails fall back to images */ }
}

// ---------------------------------------------------------------- form

async function selectArtifact(id) {
  if (!id) {
    state.current = null;
    state.idTouched = false;
    for (const f of ['title', 'id', 'tags', 'description', 'changelog', 'body']) $(f).value = '';
    $('id').readOnly = false;
    $('version').value = '0.1.0';
    $('freeSample').checked = false;
    setCaps(['sample']);
    state.cover = null;
    state.gallery = [];
    state.original = null;
    state.hasPageDraft = false;
    if (state.html) fillTitleFromHTML();
    renderMedia();
    return;
  }
  const detail = await api(`/studio/artifacts/${enc(id)}`);
  const a = detail.artifact;
  state.current = detail;
  $('title').value = a.title;
  $('id').value = a.id;
  $('id').readOnly = true;
  $('tags').value = (a.tags || []).join(', ');
  $('description').value = a.description || '';
  $('changelog').value = '';
  const releases = [...(detail.releases || [])].sort((x, y) => String(y.createdAt).localeCompare(String(x.createdAt)));
  $('version').value = nextVersion(releases.map((r) => r.version));
  setCaps(releases[0] ? manifestKeys(releases[0].manifest) : ['sample']);
  $('freeSample').checked = Boolean(releases[0]?.freeSample);
  const page = a.pageDraft || a.page || {};
  $('body').value = page.body || '';
  state.cover = page.cover ? { id: page.cover } : null;
  state.gallery = (page.gallery || []).map((gid) => ({ id: gid }));
  state.original = pageJSON(page.body || '', page.cover || '', page.gallery || []);
  state.hasPageDraft = Boolean(a.pageDraft);
  renderMedia();
}

async function pickHTML(file) {
  state.html = file;
  $('html-drop').classList.add('filled');
  $('html-picked').innerHTML = `<div class="file"><span>${esc(file.name)}</span><span class="muted">${formatBytes(file.size)}</span></div>`;
  if (state.current) return;
  // A file named like an existing artifact is probably its next version.
  const guess = slugify(file.name.replace(/\.html?$/i, ''));
  if (!$('title').value && [...$('artifact').options].some((o) => o.value && o.value === guess)) {
    $('artifact').value = guess;
    return selectArtifact(guess);
  }
  if (!$('title').value) await fillTitleFromHTML();
}

async function fillTitleFromHTML() {
  const title = htmlTitle(await state.html.slice(0, 64 << 10).text()) || state.html.name.replace(/\.html?$/i, '');
  if ($('title').value) return;
  $('title').value = title.slice(0, 120);
  if (!state.idTouched) $('id').value = slugify(title);
}

function setCaps(keys) {
  for (const box of $('caps').querySelectorAll('input')) box.checked = keys.includes(box.value);
}

const checkedCaps = () => [...$('caps').querySelectorAll('input:checked')].map((b) => b.value);

function dropZone(zone, onFiles) {
  const input = document.createElement('input');
  input.type = 'file';
  input.hidden = true;
  input.multiple = zone.hasAttribute('data-multiple');
  if (zone.dataset.accept) input.accept = zone.dataset.accept;
  zone.append(input);
  zone.addEventListener('click', (e) => { if (!e.target.closest('.x')) input.click(); });
  input.addEventListener('change', () => { if (input.files.length) onFiles([...input.files]); input.value = ''; });
  zone.addEventListener('dragover', (e) => { e.preventDefault(); zone.classList.add('over'); });
  zone.addEventListener('dragleave', () => zone.classList.remove('over'));
  zone.addEventListener('drop', (e) => {
    e.preventDefault();
    zone.classList.remove('over');
    const files = [...e.dataTransfer.files];
    if (files.length) onFiles(input.multiple ? files : files.slice(0, 1));
  });
}

function renderMedia() {
  thumbs($('cover-thumbs'), state.cover ? [state.cover] : [], () => { state.cover = null; renderMedia(); });
  thumbs($('gallery-thumbs'), state.gallery, (i) => { state.gallery.splice(i, 1); renderMedia(); });
}

function thumbs(box, items, remove) {
  box.replaceChildren(...items.map((item, i) => {
    const el = document.createElement('div');
    el.className = 'thumb';
    const src = item.file ? (item.url ||= URL.createObjectURL(item.file)) : `${SERVER}/media/${enc(item.id)}`;
    const type = item.file ? item.file.type : state.mediaTypes.get(item.id) || 'image/';
    const media = document.createElement(type.startsWith('video/') ? 'video' : 'img');
    media.src = src;
    if (media.tagName === 'VIDEO') media.muted = true;
    const x = document.createElement('button');
    x.type = 'button';
    x.className = 'x';
    x.textContent = '✕';
    x.addEventListener('click', (e) => { e.stopPropagation(); remove(i); });
    el.append(media, x);
    return el;
  }));
}

function renderAttachments() {
  $('attach-picked').replaceChildren(...state.attachments.map((f, i) => {
    const row = document.createElement('div');
    row.className = 'file';
    row.innerHTML = `<span>${esc(f.name)}</span><span class="muted">${formatBytes(f.size)}</span>`;
    const x = document.createElement('button');
    x.type = 'button';
    x.className = 'x';
    x.textContent = '✕';
    x.addEventListener('click', (e) => { e.stopPropagation(); state.attachments.splice(i, 1); renderAttachments(); });
    row.append(x);
    return row;
  }));
}

async function togglePreview() {
  const preview = $('preview');
  if (!preview.hidden) {
    preview.hidden = true;
    $('body').hidden = false;
    $('preview-toggle').textContent = 'Preview';
    return;
  }
  try {
    const { html } = await api('/studio/markdown', { method: 'POST', json: { markdown: $('body').value } });
    // The site renders relative links to its own media; point them there.
    preview.innerHTML = html.replace(/(src|href|poster)="\//g, `$1="${SERVER}/`);
  } catch (e) {
    preview.textContent = `Preview failed: ${e.message}`;
  }
  preview.hidden = false;
  $('body').hidden = true;
  $('preview-toggle').textContent = 'Edit';
}

// ---------------------------------------------------------------- upload

function collect() {
  const v = {
    id: $('id').value.trim(),
    version: $('version').value.trim(),
    title: $('title').value.trim(),
    description: $('description').value.trim(),
    changelog: $('changelog').value.trim(),
    tags: parseTags($('tags').value),
    freeSample: $('freeSample').checked,
    manifest: buildManifest(checkedCaps()),
    body: $('body').value,
  };
  const problems = [];
  for (const f of ['id', 'version', 'title']) $(f).classList.remove('invalid');
  const bad = (field, msg) => { $(field).classList.add('invalid'); problems.push(msg); };
  if (!state.html) problems.push('Choose the HTML file.');
  if (!v.title) bad('title', 'Give it a title.');
  if (!validID(v.id)) bad('id', 'The artifact ID must be lowercase letters, digits, - or _.');
  if (!validVersion(v.version)) bad('version', 'The version must be letters, digits, . _ + or -.');
  if (state.current?.releases?.some((r) => r.version === v.version)) bad('version', `Version ${v.version} already exists; pick a new one.`);
  if (!state.current && [...$('artifact').options].some((o) => o.value === v.id)) {
    bad('id', `${v.id} already exists; pick it in the Artifact list to upload a new version.`);
  }
  const media = [state.cover, ...state.gallery].filter((m) => m?.file);
  for (const m of media) if (m.file.size > state.limits.maxMediaBytes) problems.push(`${m.file.name} is larger than ${formatBytes(state.limits.maxMediaBytes)}.`);
  for (const f of state.attachments) if (f.size > state.limits.maxAttachmentBytes) problems.push(`${f.name} is larger than ${formatBytes(state.limits.maxAttachmentBytes)}.`);
  if (state.gallery.length > 24) problems.push('The gallery holds at most 24 items.');
  if (problems.length) throw new Error(problems.join(' '));
  return v;
}

const releaseForm = (v) => formData({
  artifactId: v.id, version: v.version, title: v.title, description: v.description,
  changelog: v.changelog, tags: v.tags.join(','), freeSample: v.freeSample,
  manifestJson: JSON.stringify(v.manifest), html: state.html,
});

const pageJSON = (body, cover, gallery) => JSON.stringify({ body, cover, gallery });

async function run(mode) {
  if (state.busy) return;
  $('result').hidden = false;
  $('log').replaceChildren();
  $('links').replaceChildren();
  let v;
  try {
    v = collect();
  } catch (e) {
    step(e.message, 'bad');
    return;
  }
  setBusy(true);
  let release;
  try {
    const checked = await task('Checking the HTML', () => api('/studio/releases/check', { method: 'POST', form: releaseForm(v) }));
    showChecks(checked.checks);
    if (checked.blocking) return step('Fix the errors above and try again.', 'bad');
    if (mode === 'check') return step('Looks good: ready to upload.', 'ok');

    // Media first: it needs no artifact, and a failure leaves no half-made release.
    const cover = await uploadMedia(state.cover, v.title);
    const gallery = [];
    for (const g of state.gallery) gallery.push(await uploadMedia(g, v.title));

    release = await task(`Uploading ${v.id} ${v.version}`, () => api('/studio/releases', { method: 'POST', form: releaseForm(v) }));
    for (const f of state.attachments) {
      await task(`Attaching ${f.name}`, () => api(`/studio/releases/${enc(release.id)}/attachments`, { method: 'POST', form: formData({ label: '', file: f }) }));
    }
    await task('Saving title, description and tags', () => api(`/studio/artifacts/${enc(v.id)}`, {
      method: 'PATCH', json: { title: v.title, description: v.description, tags: v.tags },
    }));

    const page = { body: v.body, cover: cover || '', gallery };
    const changed = pageJSON(page.body, page.cover, page.gallery) !== (state.original ?? pageJSON('', '', []));
    if (changed) await task('Saving the storefront page', () => api(`/studio/artifacts/${enc(v.id)}/draft`, { method: 'PUT', json: page }));
    if (mode === 'publish' && (changed || state.hasPageDraft)) {
      await task('Publishing the storefront page', () => api(`/studio/artifacts/${enc(v.id)}/draft/publish`, { method: 'POST' }));
    }
    if (mode === 'publish') await task(`Publishing ${v.version}`, () => api(`/studio/releases/${enc(release.id)}/publish`, { method: 'POST' }));

    step(mode === 'publish' ? `${v.title} ${v.version} is live.` : `${v.title} ${v.version} is saved as a draft.`, 'ok');
    links(v.id, mode);
    resetAfterUpload();
    // Reload so the next upload starts from what is now on the site.
    loadArtifacts(v.id).catch((e) => step(`Uploaded, but reloading the form failed: ${e.message}`, 'warning'));
  } catch (e) {
    if (!e.logged) step(e.message, 'bad');
    if (e.data?.checks) showChecks(e.data.checks);
    if (release) {
      step(`The release ${v.version} exists as a draft: finish or delete it in the studio.`, 'warning');
      links(v.id, 'draft');
    }
  } finally {
    setBusy(false);
  }
}

async function uploadMedia(item, alt) {
  if (!item) return '';
  if (item.id) return item.id;
  const m = await task(`Uploading ${item.file.name}`, () => api('/studio/media', { method: 'POST', form: formData({ alt: alt.slice(0, 300), file: item.file }) }));
  item.id = m.id; // a retry reuses it
  state.mediaTypes.set(m.id, m.mediaType);
  return m.id;
}

function resetAfterUpload() {
  state.html = null;
  state.attachments = [];
  $('html-drop').classList.remove('filled');
  $('html-picked').replaceChildren();
  renderAttachments();
}

// ---------------------------------------------------------------- log

async function task(label, fn) {
  const li = step(`${label}…`, 'running');
  try {
    const out = await fn();
    li.className = 'ok';
    li.textContent = label;
    return out;
  } catch (e) {
    li.className = 'bad';
    li.textContent = `${label}: ${e.message}`;
    e.logged = true;
    throw e;
  }
}

function step(text, cls) {
  const li = document.createElement('li');
  li.className = cls;
  li.textContent = text;
  $('log').append(li);
  return li;
}

function showChecks(checks = []) {
  for (const c of checks) step(c.message, c.level === 'error' ? 'bad' : c.level === 'warning' ? 'warning' : 'running');
}

function links(id, mode) {
  const out = [];
  if (mode === 'publish') out.push([`${SERVER}/apps/${enc(id)}`, 'Open the storefront page']);
  out.push([`${SERVER}/studio/a/${enc(id)}?tab=releases`, mode === 'publish' ? 'Open in the studio' : 'Preview and publish in the studio']);
  $('links').replaceChildren(...out.map(([href, text]) => {
    const a = document.createElement('a');
    a.href = href;
    a.target = '_blank';
    a.rel = 'noopener';
    a.textContent = `${text} ↗`;
    return a;
  }));
}

function setBusy(busy) {
  state.busy = busy;
  for (const b of ['check', 'draft', 'publish']) $(b).disabled = busy;
}

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}

init();
