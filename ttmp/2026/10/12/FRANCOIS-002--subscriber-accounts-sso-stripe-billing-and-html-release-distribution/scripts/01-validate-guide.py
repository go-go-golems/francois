#!/usr/bin/env python3
"""Validate ticket document structure, imported-source hashes, and observed references."""
from pathlib import Path
import hashlib
import json
import re
import yaml

root = Path(__file__).resolve().parents[1]
guide = root / 'design-doc/01-subscription-service-analysis-design-and-intern-implementation-guide.md'
text = guide.read_text()
metadata = yaml.safe_load(text.split('---', 2)[1])
assert metadata['Ticket'] == 'FRANCOIS-002'
assert metadata['DocType'] == 'design-doc'
assert text.count('```') % 2 == 0
assert re.findall(r'^## (\d+)\.', text, re.M) == [str(i) for i in range(1, 21)]
assert '<!--' not in text
assert 'proposed design and implementation guide' in text
manifest = json.loads((root / 'sources/manifest.json').read_text())
for item in manifest['files']:
    data = (root / item['snapshot']).read_bytes()
    assert len(data) == item['bytes']
    assert hashlib.sha256(data).hexdigest() == item['sha256']
for local in [
    'cmd/francois/cmds/commands.go', 'cmd/francois/cmds/update.go',
    'pkg/store/store.go', 'pkg/host/server.go', 'examples/polis.html',
]:
    assert (root.parents[4] / local).is_file(), local
prior = Path('/home/manuel/code/wesen/2026-07-25--zitadel-go-test')
for file in [
    'internal/billing/stripe.go', 'internal/billing/stripe_test.go',
    'internal/store/postgres/billing.go', 'cmd/todo-demo/serve.go',
    'cmd/todo-demo/session_store.go', 'internal/app/csrf.go',
]:
    assert (prior / file).is_file(), file
for target in re.findall(r'\]\(([^)]+)\)', text):
    if not target.startswith(('https://', 'http://')):
        assert (guide.parent / target).exists(), target
print(f'PASS: {len(text.split())} words; 20 sections; balanced fences; '
      f'{len(manifest["files"])} preserved source hashes; existing-code references.')
