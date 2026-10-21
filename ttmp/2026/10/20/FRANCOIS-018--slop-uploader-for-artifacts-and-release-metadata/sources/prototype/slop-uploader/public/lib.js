// Pure helpers shared by the page and the tests.

// Artifact IDs: lowercase letters, digits, - or _, starting with a letter or
// digit, at most 64 characters (the catalog's rule).
export function slugify(title) {
  return title
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+/, '')
    .slice(0, 64)
    .replace(/-+$/, '');
}

export const validID = (id) => /^[a-z0-9][a-z0-9_-]{0,63}$/.test(id);
export const validVersion = (v) => /^[A-Za-z0-9][A-Za-z0-9._+-]{0,39}$/.test(v);

// The next version after the given ones: the highest, with its last number
// raised by one. "0.1.0" when there are none.
export function nextVersion(versions) {
  const sorted = versions.filter(Boolean).sort(compareVersions);
  const last = sorted.at(-1);
  if (!last) return '0.1.0';
  const m = last.match(/^(.*?)(\d+)(\D*)$/);
  return m ? `${m[1]}${Number(m[2]) + 1}` : `${last}.1`;
}

function compareVersions(a, b) {
  const pa = a.split(/[^0-9]+/).map(Number);
  const pb = b.split(/[^0-9]+/).map(Number);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] || 0) - (pb[i] || 0);
    if (d) return d;
  }
  return a.localeCompare(b);
}

// Capabilities offered in the form, in the catalog's manifest shape.
export const CAPABILITIES = [
  { key: 'sample', label: 'Model access', hint: 'call the AI model (window.claude.sample)', value: {} },
  { key: 'images', label: 'Images to the model', hint: 'send up to four images with a prompt', value: null },
  { key: 'db', label: 'Shared database', hint: 'JSON documents saved per workspace', value: {} },
  { key: 'room', label: 'Rooms & presence', hint: 'live multi-user events', value: {} },
  { key: 'user', label: 'User profile', hint: "the viewer's name", value: { scopes: ['profile'] } },
  { key: 'assets', label: 'File uploads', hint: 'images and files stored with the workspace', value: {} },
  { key: 'comments', label: 'Comments', hint: 'threaded comments', value: {} },
  { key: 'downloads', label: 'Downloads', hint: 'let the app save files', value: true },
];

// buildManifest turns the ticked keys into a manifest object. "images"
// is a sample option, so it implies model access.
export function buildManifest(keys) {
  const on = new Set(keys);
  const manifest = {};
  for (const c of CAPABILITIES) {
    if (c.key === 'images' || !on.has(c.key)) continue;
    manifest[c.key] = structuredClone(c.value);
  }
  if (on.has('images')) manifest.sample = { ...(manifest.sample || {}), images: true };
  return manifest;
}

// manifestKeys is the reverse, for prefilling the form from a release.
export function manifestKeys(manifest) {
  if (!manifest || typeof manifest !== 'object') return ['sample'];
  const keys = Object.keys(manifest).filter((k) => CAPABILITIES.some((c) => c.key === k));
  if (manifest.sample && manifest.sample.images) keys.push('images');
  return keys;
}

export function htmlTitle(html) {
  const m = html.match(/<title[^>]*>([\s\S]*?)<\/title>/i);
  if (!m) return '';
  return decodeEntities(m[1]).replace(/\s+/g, ' ').trim();
}

function decodeEntities(s) {
  return s
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;|&apos;/g, "'");
}

export const parseTags = (s) =>
  [...new Set(s.split(',').map((t) => t.trim().toLowerCase()).filter(Boolean))];

export function formatBytes(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
