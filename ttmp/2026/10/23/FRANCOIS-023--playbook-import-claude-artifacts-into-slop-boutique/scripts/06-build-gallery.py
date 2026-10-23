"""06-build-gallery.py plan.json outdir [--lossy] — convert the planned images to WebP (max 2400 px wide).

plan.json: {"<slug>": [["path/to/image.png", "alt text"], ...]} — the first image is the cover.
Lossless by default: the server re-encodes resized variants, and interface
screenshots (text, thin lines) degrade visibly when compressed twice (FRANCOIS-021).
"""
import json, sys, pathlib, hashlib
from PIL import Image
plan = json.loads(pathlib.Path(sys.argv[1]).read_text()); out = pathlib.Path(sys.argv[2]); lossy = '--lossy' in sys.argv[3:]
manifest = {}
for slug, items in plan.items():
    d = out / slug; d.mkdir(parents=True, exist_ok=True); rows = []
    for i, (src, alt) in enumerate(items):
        dst = d / f"{i:02d}-{pathlib.Path(src).stem}.webp"
        im = Image.open(src).convert('RGB')
        if im.width > 2400: im = im.resize((2400, round(im.height*2400/im.width)), Image.LANCZOS)
        im.save(dst, 'WEBP', quality=86, method=6) if lossy else im.save(dst, 'WEBP', lossless=True, method=6)
        rows.append({"file": str(dst), "alt": alt, "size": im.size, "bytes": dst.stat().st_size})
    manifest[slug] = rows
(out / 'manifest.json').write_text(json.dumps(manifest, indent=1))
for s, r in manifest.items(): print(s, len(r), sum(x['bytes'] for x in r)//1024, 'KiB', max(x['bytes'] for x in r)//1024)
