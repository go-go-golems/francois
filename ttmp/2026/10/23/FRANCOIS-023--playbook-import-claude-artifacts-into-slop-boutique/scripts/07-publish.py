"""Upload gallery media, set catalog page and metadata, publish the page and
draft releases. Idempotent: a receipt records uploaded media by file.

usage: python3 -I 07-publish.py <plan.json> <gallery-manifest.json> <pages-dir> <receipt.json> [slug...]

plan.json: {"<slug>": {"description": "...", "tags": ["a","b"], "release": "rel_… or null"}}
gallery-manifest.json: output of 06-build-gallery.py (first image = cover)
pages-dir: <slug>.md page bodies (Markdown, no title heading: the page shows it)
"""
import json, pathlib, subprocess, sys

plan = json.loads(pathlib.Path(sys.argv[1]).read_text())
man = json.loads(pathlib.Path(sys.argv[2]).read_text())
pages = pathlib.Path(sys.argv[3])
receipt_f = pathlib.Path(sys.argv[4])
only = sys.argv[5:]
receipt = json.loads(receipt_f.read_text()) if receipt_f.exists() else {}

def cli(*a):
    r = subprocess.run(['francois', 'catalog', *a, '--format', 'json'], capture_output=True, text=True)
    if r.returncode:
        raise SystemExit(' '.join(a[:3]) + ': ' + r.stderr[-800:])
    out = json.loads(r.stdout or '[]')
    return out[0] if isinstance(out, list) and out else out

def save():
    receipt_f.write_text(json.dumps(receipt, indent=1))

for slug, rows in man.items():
    if only and slug not in only:
        continue
    meta = plan[slug]
    st = receipt.setdefault(slug, {'media': {}})
    cur = cli('artifacts', 'show', slug)
    assert not cur['artifact'].get('pageDraft'), slug + ' has an unpublished page draft; do not overwrite it'
    ids = []
    for r in rows:
        if r['file'] not in st['media']:
            m = cli('media', 'upload', r['file'], '--alt', r['alt'])
            st['media'][r['file']] = m['id']
            save()
            print('uploaded', slug, pathlib.Path(r['file']).name, m['id'], flush=True)
        ids.append(st['media'][r['file']])
    cli('artifacts', 'update', slug, '--description', meta['description'], '--tags', ','.join(meta['tags']))
    cli('artifacts', 'page-set', slug, '--body-file', str(pages / (slug + '.md')), '--cover', ids[0], '--gallery', ','.join(ids[1:]))
    cli('artifacts', 'page-publish', slug)
    rel_id = meta.get('release')
    if rel_id:
        rel = next(x for x in cur['releases'] if x['id'] == rel_id)
        if rel['state'] == 'draft':
            cli('releases', 'publish', rel_id)
            print('published release', slug, rel_id, flush=True)
    v = cli('artifacts', 'show', slug)
    assert v['artifact']['page']['cover'] == ids[0] and v['artifact']['page']['gallery'] == ids[1:], slug
    st.update(cover=ids[0], gallery=ids[1:], releases=[(x['version'], x['state'], x.get('downloadAccess')) for x in v['releases']])
    save()
    print('done', slug, st['releases'], flush=True)
