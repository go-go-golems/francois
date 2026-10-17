"""Publish curated screenshots and public access using only the Francois CLI."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

TICKET = Path(__file__).resolve().parents[1]
REPO = next(p for p in TICKET.parents if (p / 'go.mod').is_file())
parser = argparse.ArgumentParser()
parser.add_argument('--token-file', required=True)
parser.add_argument('--admin-token-file', help='Also enable bounded tours; requires explicit administrative approval')
parser.add_argument('--artifact', help='Update only this artifact when refining a gallery')
args = parser.parse_args()
BASE = 'https://slop.boutique'
plan = json.loads((TICKET / 'sources/showcase.json').read_text())
receipt_file = TICKET / 'various/gallery-publication-receipt.json'
receipt = json.loads(receipt_file.read_text()) if receipt_file.exists() else {'base': BASE, 'artifacts': {}}


def save():
    receipt_file.write_text(json.dumps(receipt, indent=2) + '\n')


def cli(*command, admin=False):
    token = args.admin_token_file if admin else args.token_file
    cmd = ['go', 'run', './cmd/francois', 'catalog', *command,
           '--server', BASE, '--token-file', token, '--format', 'json']
    result = subprocess.run(cmd, cwd=REPO, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError('CLI ' + ' '.join(command[:2]) + ': ' + result.stderr[-1200:])
    rows = json.loads(result.stdout)
    return rows if isinstance(rows, list) else [rows]


for app in plan['artifacts']:
    slug = app['id']
    if args.artifact and slug != args.artifact:
        continue
    current = cli('artifacts', 'show', slug)[0]
    if current['artifact'].get('pageDraft'):
        raise RuntimeError(slug + ' has an existing unpublished page draft; preserve it')
    state = receipt['artifacts'].setdefault(slug, {'images': {}})
    for image in app['images']:
        local = TICKET / 'various/showcase' / image['file']
        assert hashlib.sha256(local.read_bytes()).hexdigest() == image['sha256']
        previous = state['images'].get(image['file'])
        if previous and previous['sha256'] == image['sha256']:
            continue
        media = cli('media', 'upload', str(local), '--alt', image['alt'])[0]
        state['images'][image['file']] = {'id': media['id'], 'sha256': image['sha256'], 'alt': image['alt']}
        save()
        print('Uploaded', slug, image['file'], media['id'], flush=True)
    media_ids = [state['images'][image['file']]['id'] for image in app['images']]
    # Preserve the published Markdown; only cover/gallery are changed.
    body_dir = TICKET / 'various/page-bodies'
    body_dir.mkdir(exist_ok=True)
    body = body_dir / (slug + '.txt')
    body.write_text(current['artifact']['page']['body'])
    cli('artifacts', 'page-set', slug, '--body-file', str(body), '--cover', media_ids[0], '--gallery', ','.join(media_ids[1:]))
    cli('artifacts', 'page-publish', slug)
    cli('artifacts', 'update', slug, '--visibility', 'listed', '--required-plan', 'free', '--demo-enabled', 'true')
    # Public download access belongs to the immutable release, so publish a
    # new version of the identical HTML with the free-sample flag enabled.
    version = '0.1.2'
    existing = next((r for r in current['releases'] if r['version'] == version), None)
    if existing:
        html = next(f for f in existing['files'] if f['role'] == 'html')
        if html['sha256'] != app['releaseHTMLSHA256'] or existing.get('downloadAccess') != 'free_sample':
            raise RuntimeError(slug + ' already has a different ' + version)
        if existing['state'] == 'draft':
            cli('releases', 'publish', existing['id'])
        release_id = existing['id']
    else:
        directory = REPO / 'examples/imports' / slug
        result = cli('releases', 'push', '--artifact-id', slug, '--version', version,
                     '--html', str(directory / 'release.html'), '--manifest', str(directory / 'manifest.json'),
                     '--free-sample', '--publish', '--changelog', 'Public download access; application HTML is unchanged.')[0]
        release_id = result['release_id']
    verified = cli('artifacts', 'show', slug)[0]
    assert verified['artifact']['page']['cover'] == media_ids[0]
    assert verified['artifact']['page']['gallery'] == media_ids[1:]
    state.update({'cover': media_ids[0], 'gallery': media_ids[1:], 'releaseId': release_id,
                  'version': version, 'publicDownloads': True, 'demoEnabled': True,
                  'releaseHTMLSHA256': app['releaseHTMLSHA256']})
    save()
    print('Published gallery/public release', slug, len(media_ids), flush=True)

if args.admin_token_file:
    # The user explicitly approved the separate admin token for this setting.
    current = cli('settings', 'show', admin=True)[0]
    doc = current['document']
    before = json.loads(json.dumps(doc))
    if not doc['demo']['enabled']:
        doc['demo']['enabled'] = True
        settings_file = Path('/tmp/francois-009-private/tours-settings.json')
        settings_file.write_text(json.dumps(doc))
        settings_file.chmod(0o600)
        cli('settings', 'apply', '--file', str(settings_file), '--base', str(current['revision']), '--yes', admin=True)
    after = cli('settings', 'show', admin=True)[0]
    expected = json.loads(json.dumps(before))
    expected['demo']['enabled'] = True
    assert after['document'] == expected, 'Only demo.enabled may change'
    receipt.setdefault('tours', {'beforeRevision': current['revision'], 'revision': after['revision'],
                                'before': before['demo'], 'after': after['document']['demo'],
                                'onlyDemoEnabledChanged': True})
    save()
    print('Enabled bounded public tours, revision', after['revision'], flush=True)
