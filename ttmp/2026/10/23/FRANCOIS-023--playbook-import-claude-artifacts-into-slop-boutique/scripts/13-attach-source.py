"""Attach a zip to an artifact's newest published release as a download.

usage: python3 -I 13-attach-source.py <slug> <zip> [label]     (default label "Source (zip)")

Skips the upload when the release already has an attachment with the same
file name and size. Attachments follow the release's download rule, so on a
free sample anyone can download them. Retries transient network errors.
"""
import json, os, pathlib, subprocess, sys, time
slug, zf = sys.argv[1], pathlib.Path(sys.argv[2]).resolve()
label = sys.argv[3] if len(sys.argv) > 3 else 'Source (zip)'
repo = os.environ.get('FRANCOIS_REPO', '/home/manuel/code/wesen/go-go-golems/francois')

def cli(*a):
    for attempt in range(5):
        r = subprocess.run(['francois', 'catalog', *a, '--format', 'json'], capture_output=True, text=True, cwd=repo)
        if not r.returncode or not any(k in r.stderr for k in ('tls:', 'connection reset', 'EOF', 'timeout', 'Timeout', 'deadline exceeded')):
            break
        time.sleep(2 + 3 * attempt)
    if r.returncode:
        raise SystemExit(' '.join(a[:3]) + ': ' + (r.stderr or r.stdout)[-800:])
    out = json.loads(r.stdout or '[]')
    return out[0] if isinstance(out, list) and out else out

rels = [r for r in cli('artifacts', 'show', slug)['releases'] if r['state'] == 'published']
assert rels, slug + ' has no published release'
rel = max(rels, key=lambda r: r['publishedAt'])
if any(a.get('name') == zf.name and a.get('size') == zf.stat().st_size for a in rel.get('attachments') or []):
    print(slug, 'already attached', zf.name)
else:
    a = cli('attachments', 'add', rel['id'], str(zf), '--label', label)
    print(slug, 'attached', zf.name, a.get('id'), 'to', rel['id'], rel['version'])
