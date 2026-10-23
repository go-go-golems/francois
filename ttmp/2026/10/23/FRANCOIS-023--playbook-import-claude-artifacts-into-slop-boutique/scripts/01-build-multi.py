"""Bundle a multi-file Claude artifact (js/ ES modules + assets/*.json GLB
models) into one HTML release for slop.boutique.

- esbuild bundles js/main.js; three, htm and https:// imports stay external
  (the page's importmap still resolves them from the CDN).
- Each assets/**/*.json model (GLB bytes in base64) is decoded, gzipped and
  re-encoded into one JSON block; the single fetch in assets.js:loadModel is
  replaced by a lookup that inflates with DecompressionStream.

usage: python3 -I build_multi.py <src-dir> <out.html> <esbuild-binary>
"""
import base64, gzip, json, pathlib, re, shutil, subprocess, sys, tempfile

CREDENTIAL = re.compile(r"(?i)(sk_(live|test)_[A-Za-z0-9]{8,}|rk_(live|test)_[A-Za-z0-9]{8,}|whsec_[A-Za-z0-9]{8,}|AKIA[0-9A-Z]{16}|sk-ant-[A-Za-z0-9_-]{10,}|sk-[A-Za-z0-9]{32,})")
src, out, esbuild =pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), sys.argv[3]

FETCH = """  const res = await fetch(BASE + path.replace(/\\.glb$/, '.json'));
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const j = await res.json();
  const bin = atob(j.data), buf = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) buf[i] = bin.charCodeAt(i);
  return loader.parseAsync(buf.buffer, BASE);"""
PACKED = """  // slop.boutique release: models are packed into the page (see #lcs-assets).
  return loader.parseAsync(await window.__lcsAsset(path.replace(/\\.glb$/, '.json')), BASE);"""

with tempfile.TemporaryDirectory() as tmp:
    work = pathlib.Path(tmp) / "app"
    shutil.copytree(src / "js", work / "js")
    assets_js = work / "js" / "assets.js"
    text = assets_js.read_text()
    assert text.count(FETCH) == 1, "loadModel fetch block not found exactly once"
    assets_js.write_text(text.replace(FETCH, PACKED))
    bundle = pathlib.Path(tmp) / "bundle.js"
    subprocess.run([esbuild, str(work / "js" / "main.js"), "--bundle", "--format=esm",
                    "--external:three", "--external:three/*", "--external:htm",
                    "--external:https://*", "--log-level=warning", f"--outfile={bundle}"], check=True)
    js = bundle.read_text()

pack = {}
for f in sorted((src / "assets").rglob("*.json")):
    j = json.loads(f.read_bytes())
    assert j.get("format") == "glb-base64", f
    raw = base64.b64decode(j["data"])
    # Random base64 can resemble an AWS key ID, which the release validator
    # rejects as credential-like; another compression level changes the bytes.
    for level in (9, 8, 7, 6, 5):
        enc = base64.b64encode(gzip.compress(raw, level, mtime=0)).decode()
        if not CREDENTIAL.search(enc):
            break
    else:
        raise SystemExit(f"{f}: every encoding looks credential-like")
    pack[str(f.relative_to(src / "assets"))] = enc

loader = """<script>
(() => {
  const pack = JSON.parse(document.getElementById('lcs-assets').textContent);
  window.__lcsAsset = async (path) => {
    const b64 = pack[path];
    if (!b64) throw new Error('HTTP 404');
    const bin = atob(b64), gz = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) gz[i] = bin.charCodeAt(i);
    return new Response(new Blob([gz]).stream().pipeThrough(new DecompressionStream('gzip'))).arrayBuffer();
  };
})();
</script>"""
licenses = "\n".join(p.read_text().strip() for p in sorted((src / "LICENSES").glob("*.txt"))) if (src / "LICENSES").exists() else ""
notice = "<!-- 3D models: KayKit Adventurers (Kay Lousberg) and Kenney Furniture/Nature kits, CC0; animal and character models from the three.js examples. -->"

html = (src / "index.html").read_text()
tag = '<script type="module" src="./js/main.js"></script>'
assert html.count(tag) == 1
inline = ('<script type="application/json" id="lcs-assets">' + json.dumps(pack, separators=(",", ":")) + "</script>\n"
          + loader + '\n<script type="module">\n' + js.replace("</script", "<\\/script") + "\n</script>")
html = html.replace(tag, notice + "\n" + inline)
out.write_text(html)
print(f"{out}: {len(html.encode())/1e6:.2f} MB, {len(pack)} models, bundle {len(js)/1e3:.0f} kB")
