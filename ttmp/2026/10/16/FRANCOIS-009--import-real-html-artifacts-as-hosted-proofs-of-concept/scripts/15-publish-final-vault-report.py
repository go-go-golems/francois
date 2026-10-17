"""Publish a new final report and exact evidence copies without replacing notes."""
import hashlib
import json
from pathlib import Path
import shutil

ticket = Path(__file__).resolve().parents[1]
repo = next(p for p in ticket.parents if (p / 'go.mod').is_file())
speed = ticket.parent / 'FRANCOIS-010--measure-and-improve-slop-boutique-page-and-application-startup-speed'
vault = Path('/home/manuel/code/wesen/go-go-golems/go-go-parc')
directory = vault / 'Projects/2026/10/16'
note = directory / 'PROJ - Francois - From HTML Exports to Public Applications on k3s - A Technical Deep Dive.md'
assets = directory / '_assets/francois-public-html'
if note.exists() or assets.exists():
    raise SystemExit('Refusing to replace an existing report or its assets')

plan = json.loads((ticket / 'sources/showcase.json').read_text())
copies = {}
for app in plan['artifacts']:
    for image in app['images']:
        source = ticket / 'various/showcase' / image['file']
        assert hashlib.sha256(source.read_bytes()).hexdigest() == image['sha256']
        copies[source] = image['file']

evidence = [
    (ticket / 'sources/builds.json', 'builds.json', 'Exact generated HTML identity and manifests'),
    (ticket / 'sources/showcase.json', 'showcase.json', 'Original screenshot captions and identities'),
    (ticket / 'sources/showcase-web.json', 'showcase-web.json', 'Published delivery-copy identities'),
    (ticket / 'various/local-functional-receipt.json', 'local-functional.json', 'Local interaction, persistence and sampling evidence'),
    (ticket / 'various/production-receipt.json', 'production-functional.json', 'Production application/model qualification'),
    (ticket / 'various/gallery-publication-receipt.json', 'gallery-publication.json', 'Current public releases/media and tour-policy change'),
    (ticket / 'various/public-gallery-verification.json', 'public-tour-verification.json', 'Original anonymous download and RAG tour/model probe'),
    (ticket / 'various/portal-rollout-verification.json', 'page-title-verification.json', 'Single-download, title and tour-lifetime verification'),
    (ticket / 'various/web-media-optimization.json', 'web-media-optimization.json', 'Exact source/delivery byte comparison'),
    (ticket / 'various/web-media-verification.json', 'web-media-verification.json', 'Actual public delivery-byte and conditional-response checks'),
    (speed / 'various/cache-before.json', 'cache-before.json', 'Initial cold/warm browser response measurements'),
    (speed / 'various/cache-after.json', 'cache-after.json', 'Same measurements after the deployed change'),
    (speed / 'various/cache-contract-verification.json', 'cache-contract.json', 'Live public/private HTTP cache assertions'),
    (speed / 'various/production-deployment.json', 'deployment.json', 'Image, source/GitOps revisions and k3s rollout state'),
    (speed / 'various/startup-before.json', 'startup-probe.json', 'Explicit declined startup probe; no inferred latency'),
]
for source, name, _ in evidence:
    copies[source] = name
for name in ['media-transfer.png', 'warm-transfer.png']:
    copies[speed / 'various' / name] = name

report = ticket / 'analysis/01-from-html-exports-to-public-applications-on-k3s-final-technical-project-report.md'
body = report.read_text().split('---', 2)[2].lstrip('\n')
body = body.replace('../various/showcase/', '_assets/francois-public-html/')
body = body.replace('../../' + speed.name + '/various/', '_assets/francois-public-html/')
metadata = '''---
title: "Francois: From HTML Exports to Public Applications on k3s"
aliases:
  - Francois Public HTML Applications
  - FRANCOIS-009 Final Project Report
  - Francois Import and Delivery Engineering
tags:
  - project
  - go
  - html
  - performance
  - kubernetes
status: active
report_status: shipped
type: project
created: 2026-10-16
repo: /home/manuel/code/wesen/go-go-golems/francois
tickets:
  - FRANCOIS-009
  - FRANCOIS-010
source_archive_commit: 17645c6
deployed_service_commit: 393bf35d51ab467081d7a09460937df66ba446b9
gitops_commit: e3ccf7b09784bb6fadef6dd847216badee7e2f2c
production_observed_at: 2026-10-17T00:47:16Z
---

'''
assets.mkdir(parents=True)
inventory = []
for source, name in copies.items():
    shutil.copyfile(source, assets / name)
    inventory.append({'file': name, 'source': str(source.relative_to(repo)),
                      'bytes': source.stat().st_size,
                      'sha256': hashlib.sha256(source.read_bytes()).hexdigest()})
(assets / 'asset-inventory.json').write_text(json.dumps(inventory, indent=2) + '\n')
(ticket / 'various/final-project-report-assets.json').write_text(json.dumps(inventory, indent=2) + '\n')
body += '\n### Copied evidence files\n\n| Evidence | What it establishes |\n| --- | --- |\n'
body += '\n'.join('| [' + name + '](_assets/francois-public-html/' + name + ') | ' + reason + '. |' for _, name, reason in evidence)
body += '\n\n[Asset inventory](_assets/francois-public-html/asset-inventory.json) records every copied file and SHA256. '
body += 'The original anonymous-tour receipt precedes the later seventeen-image refinement; use the current publication and WebP verification receipts for current media identity.\n'
body += '\nRelated: [[PROJ - Francois - Page Speed and Browser Cache Engineering - A Technical Deep Dive]]; [[PROJ - Francois - Private Analytics and Minimal Hosting - A Technical Deep Dive]].\n'
note.write_text(metadata + body)
print(json.dumps({'note': str(note), 'copiedFiles': len(inventory), 'bytes': sum(i['bytes'] for i in inventory)}, indent=2))
