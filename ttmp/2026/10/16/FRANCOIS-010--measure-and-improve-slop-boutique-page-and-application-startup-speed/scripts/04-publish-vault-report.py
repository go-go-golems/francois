"""Create a new self-contained vault note; refuse to overwrite historical notes."""
from pathlib import Path
import shutil

ticket = Path(__file__).resolve().parents[1]
imports = ticket.parent / 'FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept'
vault = Path('/home/manuel/code/wesen/go-go-golems/go-go-parc')
directory = vault / 'Projects/2026/10/16'
note = directory / 'PROJ - Francois - Page Speed and Browser Cache Engineering - A Technical Deep Dive.md'
if note.exists():
    raise SystemExit('Refusing to overwrite existing vault history: ' + str(note))
assets = directory / '_assets'
assets.mkdir(parents=True, exist_ok=True)
source = ticket / 'analysis/01-page-speed-engineering-measured-costs-cache-design-and-implementation-guide.md'
body = source.read_text().split('---', 2)[2].lstrip('\n')
body = body.replace('../various/media-transfer.png', '_assets/francois-010-media-transfer.png')
body = body.replace('../../FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/various/showcase/catalog-public-covers.png', '_assets/francois-010-catalog.png')
copies = {
    ticket / 'various/media-transfer.png': 'francois-010-media-transfer.png',
    imports / 'various/showcase/catalog-public-covers.png': 'francois-010-catalog.png',
    ticket / 'various/cache-before.json': 'francois-010-cache-before.json',
    ticket / 'various/startup-before.json': 'francois-010-startup-before.json',
    imports / 'various/web-media-optimization.json': 'francois-010-web-media-optimization.json',
    imports / 'various/web-media-verification.json': 'francois-010-web-media-verification.json',
}
for original, name in copies.items():
    destination = assets / name
    if destination.exists():
        raise SystemExit('Refusing to overwrite existing asset: ' + str(destination))
    shutil.copyfile(original, destination)
metadata = '''---
title: "Francois: Page Speed and Browser Cache Engineering"
aliases:
  - FRANCOIS-010 Project Report
  - Francois Page Speed
tags:
  - project
  - go
  - performance
  - http
  - browser-cache
status: active
type: project
created: 2026-10-16
repo: /home/manuel/code/wesen/go-go-golems/francois
ticket: FRANCOIS-010
pr: https://github.com/go-go-golems/francois/pull/9
source_ref: b52d97b
---

'''
evidence = '\n\n### Self-contained evidence copied with this report\n\n' + '\n'.join(
    '- [' + name + '](_assets/' + name + ')' for name in copies.values() if name.endswith('.json'))
evidence += '\n\nRelated: [[PROJ - Francois - Private Analytics and Minimal Hosting - A Technical Deep Dive]].\n'
note.write_text(metadata + body + evidence)
print(note)
