"""Append the verified outcome to the ticket/vault without rewriting history."""
import json
from pathlib import Path
import shutil

ticket = Path(__file__).resolve().parents[1]
report = ticket / 'analysis/01-page-speed-engineering-measured-costs-cache-design-and-implementation-guide.md'
vault_dir = Path('/home/manuel/code/wesen/go-go-golems/go-go-parc/Projects/2026/10/16')
note = vault_dir / 'PROJ - Francois - Page Speed and Browser Cache Engineering - A Technical Deep Dive.md'
before = json.loads((ticket / 'various/cache-before.json').read_text())
after = json.loads((ticket / 'various/cache-after.json').read_text())
contract = json.loads((ticket / 'various/cache-contract-verification.json').read_text())
deployment = json.loads((ticket / 'various/production-deployment.json').read_text())
marker = '### Production checkpoint: 16 October 2026'
for destination in [report, note]:
    if marker in destination.read_text():
        raise SystemExit('Outcome already exists: ' + str(destination))

def visit(receipt, route, phase):
    return next(v for v in receipt['visits'] if v['route'] == route and v['pass'] == phase)

lines = [marker, '',
    'The focused source change is deployed from PR #9, merge ' + deployment['sourceMerge'] + '. '
    'Image workflow 38987577959 published ' + deployment['image'] + '; '
    '[GitOps PR #361](https://github.com/wesen/2026-03-27--hetzner-k3s/pull/361) changed only that image pin and merged as ' + deployment['gitOpsMerge'] + '. '
    'The deployment has one desired, updated, ready and available replica at generation 13. Argo CD reports Synced, Healthy and Succeeded. '
    'No settings or schema change was required.', '',
    'The repeated browser test and the live contract verifier both passed. The exact same portal JavaScript bytes now transfer as '
    f"{contract['assets'][0]['gzipBytes']:,} gzip bytes instead of {contract['assets'][0]['identityBytes']:,} identity bytes. "
    'All measured warm portal/static requests transfer zero response bytes. A conditional request returns a bodyless 304, HEAD returns no body, '
    'and unversioned URLs still require revalidation. The session API remains no-store. '
    'On a known existing instance origin, the launch document remains no-store, its script is versioned, the common runtime is immutable/gzipped, '
    'and an unauthorized /page request returns 401 with no-store. No tour or model call is needed for these checks.', '',
    '| Route | Cold observed wire bytes, before → after | Warm observed wire bytes, before → after |',
    '| --- | ---: | ---: |']
for route in ['/apps', '/apps/lattice-deck', '/account']:
    values = []
    for phase in ['cold', 'warm']:
        values.append(' → '.join(f"{sum(r.get('wireBytes', 0) for r in visit(receipt, route, phase)['responses']):,}" for receipt in [before, after]))
    lines.append(f'| `{route}` | {values[0]} | {values[1]} |')
lines += ['', '![Measured warm-navigation response transfer](../various/warm-transfer.png)', '',
    'The elapsed-time observations remain a separate result:', '',
    '| Route | Cold load event, before → after | Warm load event, before → after |',
    '| --- | ---: | ---: |']
for route in ['/apps', '/apps/lattice-deck', '/account']:
    values = []
    for phase in ['cold', 'warm']:
        values.append(' → '.join(f"{visit(receipt, route, phase)['navigation'][0]['loadMs']:.0f} ms" for receipt in [before, after]))
    lines.append(f'| `{route}` | {values[0]} | {values[1]} |')
lines += ['',
    'The warm samples improved on all three routes. Cold elapsed times did not improve uniformly: the account load took longer in the later sample, '
    'even with substantially fewer bytes. These single observations cannot separate network variation, connection setup, browser scheduling and execution. '
    'The verified result is the lower transfer requirement and correct reuse; this report does not turn that into an unsupported latency guarantee. '
    'An end-to-end application-start speedup remains unmeasured because the baseline tour probe was declined by the existing network limit. '
    'Smaller catalog thumbnails, route-specific bundles and app dependency packaging remain conditional follow-ups after a permitted startup profile.', '']
outcome = '\n'.join(lines).rstrip() + '\n'
(ticket / 'various/production-outcome.txt').write_text(outcome)

copies = {
    'cache-after.json': 'francois-010-cache-after.json',
    'cache-contract-verification.json': 'francois-010-cache-contract-verification.json',
    'production-deployment.json': 'francois-010-production-deployment.json',
    'warm-transfer.png': 'francois-010-warm-transfer.png',
}
for original, name in copies.items():
    destination = vault_dir / '_assets' / name
    if destination.exists():
        raise SystemExit('Refusing to overwrite evidence: ' + str(destination))
    shutil.copyfile(ticket / 'various' / original, destination)
with report.open('a') as output:
    output.write('\n' + outcome)
vault_outcome = outcome.replace('../various/warm-transfer.png', '_assets/francois-010-warm-transfer.png')
vault_outcome += '\nFinal evidence: ' + ', '.join('[' + name + '](_assets/' + name + ')' for name in copies.values() if name.endswith('.json')) + '.\n'
with note.open('a') as output:
    output.write('\n' + vault_outcome)
print('Appended verified outcome to ticket and vault report')
