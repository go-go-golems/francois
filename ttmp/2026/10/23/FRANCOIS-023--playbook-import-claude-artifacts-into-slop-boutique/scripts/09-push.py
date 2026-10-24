"""Push prepared slugs as draft releases and collect the inputs for 06/07.

usage: python3 -I 09-push.py <imp-dir> <early-access-until RFC3339 | -> [--no-free-sample] slug...

<imp-dir>/<slug>/ must hold the per-slug deliverables from the agent brief
(reference/02-agent-brief-template.md): build/<slug>.html, build/<slug>.json,
gallery.json, page.md, meta.json. Writes <imp-dir>/pub/{plan.json,gallery.json,pages/}.

Per slug: `releases push` 0.1.0 (free sample unless --no-free-sample), looks up the
draft's id (push prints none), sets early access ("-" leaves it unset; the site
default earlyAccessDays is 0). A slug already in plan.json is not pushed again,
but its gallery and page text are refreshed, so re-run it after redoing shots.
"""
import json, os, pathlib, subprocess, sys, shutil
args = [a for a in sys.argv[1:] if not a.startswith('--')]
free = '--no-free-sample' not in sys.argv
imp = pathlib.Path(args[0]).resolve(); until = args[1]; pub = imp / 'pub'
(pub / 'pages').mkdir(parents=True, exist_ok=True)
repo = os.environ.get('FRANCOIS_REPO', '/home/manuel/code/wesen/go-go-golems/francois')
plan_f = pub / 'plan.json'; plan = json.loads(plan_f.read_text()) if plan_f.exists() else {}
gal_f = pub / 'gallery.json'; gal = json.loads(gal_f.read_text()) if gal_f.exists() else {}

def cli(*a):
    r = subprocess.run(['francois', 'catalog', *a, '--format', 'json'], capture_output=True, text=True, cwd=repo)
    if r.returncode:
        raise SystemExit(' '.join(a[:3]) + ': ' + (r.stderr or r.stdout)[-800:])
    out = json.loads(r.stdout or '[]')
    return out[0] if isinstance(out, list) and out else out

for slug in args[2:]:
    d = imp / slug; m = json.loads((d / 'meta.json').read_text())
    assert not m.get('blocked'), slug + ' is blocked: ' + m.get('notes', '')
    if not (slug in plan and plan[slug].get('release')):
        cli('releases', 'push', '--artifact-id', slug, '--title', m['title'], '--version', '0.1.0',
            '--html', str(d / 'build' / f'{slug}.html'), '--manifest', str(d / 'build' / f'{slug}.json'),
            *(['--free-sample'] if free else []), '--description', m['description'], '--tags', ','.join(m['tags']))
        rid = next(r['id'] for r in cli('artifacts', 'show', slug)['releases'] if r['state'] == 'draft' and r['version'] == '0.1.0')
        if until != '-':
            cli('artifacts', 'update', slug, '--early-access-until', until)
        plan[slug] = {'description': m['description'], 'tags': m['tags'], 'release': rid}
        print('pushed', slug, rid, flush=True)
    plan[slug].update(description=m['description'], tags=m['tags'])
    gal.update(json.loads((d / 'gallery.json').read_text()))
    shutil.copy(d / 'page.md', pub / 'pages' / f'{slug}.md')
    plan_f.write_text(json.dumps(plan, indent=1)); gal_f.write_text(json.dumps(gal, indent=1))
