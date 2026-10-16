"""Package the downloaded exports as single-file hosted HTML releases."""
from pathlib import Path
import hashlib
import json
import re

REPO = next(p for p in Path(__file__).resolve().parents if (p / 'go.mod').is_file())
TICKET = Path(__file__).resolve().parents[1]
ROOT = REPO / 'examples' / 'imports'
MANIFESTS = {
    'lattice-rift': {},
    'lattice-phantasmagoria': {},
    'tabula-workbench': {'sample': {}, 'downloads': True},
    'lattice-deck': {'sample': {}, 'db': {}, 'user': {'scopes': ['profile']}, 'room': {}},
    'rag-workbench': {'sample': {}, 'db': {}, 'user': {'scopes': ['profile']}, 'downloads': True},
}

def replace_once(text, old, new):
    assert text.count(old) == 1, f'Expected one occurrence: {old[:90]}'
    return text.replace(old, new, 1)

def script(text):
    return '<script>\n' + re.sub(r'</script', r'<\\/script', text, flags=re.I) + '\n</script>'

receipts = []
for slug, manifest in MANIFESTS.items():
    app = ROOT / slug
    source = app / 'source'
    html = (source / 'index.html').read_text()
    if slug == 'lattice-deck':
        # Room presence is fire-and-forget in the supported runtime.
        html, count = re.subn(r'((?:this\.lobby|this\.room)\.presence\([^;\n]+?\))\.catch', r'Promise.resolve(\1).catch', html)
        assert count == 2, count
    bundled = None
    if slug == 'rag-workbench':
        presets = json.loads((source / 'presets/index.json').read_text())
        # Start with a complete local retrieval pipeline, requiring no model call.
        presets.sort(key=lambda p: p['id'] != 'minimal')
        bundled = {
            'presets': presets,
            'presetSources': {p.name: p.read_text() for p in (source / 'presets').glob('*.js')},
            'corpus': json.loads((app / 'demo-corpus.json').read_text()),
            'ragSource': (source / 'rag.js').read_text(),
        }
        bundled['presetSources']['minimal.js'] = bundled['presetSources']['minimal.js'].replace("q: 'rebalance law'", "q: 'model concurrency'")
        packed = json.dumps(bundled, ensure_ascii=False).replace('<', '\\u003c')
        html = replace_once(html, '<script src="rag.js"></script>', '<script type="application/json" id="francois-import-bundle">' + packed + '</script>\n<script src="rag.js"></script>')
    def inline(match):
        name = match.group(1)
        if name.startswith(('https://', 'http://')):
            return match.group(0)
        path = source / name
        assert path.resolve().is_relative_to(source.resolve()) and path.is_file(), name
        code = path.read_text()
        if slug == 'rag-workbench' and name == 'app.js':
            code = replace_once(code, "  'use strict';", "  'use strict';\n  const bundled = JSON.parse(document.getElementById('francois-import-bundle').textContent);")
            code = replace_once(code, "fetch('presets/index.json').then(r => r.json()).catch(() => [])", 'Promise.resolve(bundled.presets)')
            code = replace_once(code, "    const r = await fetch('presets/' + p.file);\n    if (!r.ok) throw new Error(`could not load preset ${p.file}`);\n    return r.text();", "    const text = bundled.presetSources[p.file];\n    if (typeof text !== 'string') throw new Error(`could not load preset ${p.file}`);\n    return text;")
            code = replace_once(code, "fetch('corpus.json').then(r => r.json()).catch(() => ({ files: [] }))", 'Promise.resolve(bundled.corpus)')
            code = replace_once(code, "librarySource: () => fetch('rag.js').then(r => r.text())", 'librarySource: () => Promise.resolve(bundled.ragSource)')
        if slug == 'rag-workbench' and name == 'rag.js':
            code = replace_once(code, 'p.concurrency ?? 3', 'p.concurrency ?? 1')
        return script(code)
    html = re.sub(r'<script\s+src="([^"]+)"\s*></script>', inline, html)
    assert not re.search(r'<script\s+src="(?!https?://)', html), slug
    release = app / 'release.html'
    release.write_text(html)
    (app / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    receipts.append({'artifact': slug, 'bytes': release.stat().st_size, 'sha256': hashlib.sha256(release.read_bytes()).hexdigest(), 'manifest': manifest})
(TICKET / 'sources/builds.json').write_text(json.dumps(receipts, indent=2) + '\n')
print(json.dumps(receipts, indent=2))
