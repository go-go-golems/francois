"""Encode web delivery copies; preserve every original report PNG unchanged."""
import hashlib
import json
from pathlib import Path
import subprocess

ticket = Path(__file__).resolve().parents[1]
plan = json.loads((ticket / 'sources/showcase.json').read_text())
root = ticket / 'various/showcase'
(root / 'web').mkdir(exist_ok=True)
totals = {'pngBytes': 0, 'webpBytes': 0, 'images': []}
for app in plan['artifacts']:
    for image in app['images']:
        source = root / image['file']
        assert hashlib.sha256(source.read_bytes()).hexdigest() == image['sha256']
        destination = root / 'web' / (source.stem + '.webp')
        subprocess.run(['cwebp', '-quiet', '-q', '85', str(source), '-o', str(destination)], check=True)
        entry = {'source': image['file'], 'sourceSHA256': image['sha256'],
                 'file': str(destination.relative_to(root)), 'pngBytes': source.stat().st_size,
                 'webpBytes': destination.stat().st_size}
        totals['pngBytes'] += entry['pngBytes']
        totals['webpBytes'] += entry['webpBytes']
        totals['images'].append(entry)
        image['file'] = entry['file']
        image['sha256'] = hashlib.sha256(destination.read_bytes()).hexdigest()
(ticket / 'sources/showcase-web.json').write_text(json.dumps(plan, indent=2) + '\n')
(ticket / 'various/web-media-optimization.json').write_text(json.dumps(totals, indent=2) + '\n')
print(json.dumps({k: v for k, v in totals.items() if k != 'images'}))
