"""Copy the named Downloads exports without changing the original files."""
from pathlib import Path
import hashlib
import json
import stat
import zipfile

REPO = next(p for p in Path(__file__).resolve().parents if (p / "go.mod").is_file())
TICKET = Path(__file__).resolve().parents[1]
DOWNLOADS = Path.home() / "Downloads"
EXPORTS = {
    "lattice-rift": "Lattice Rift.zip",
    "lattice-phantasmagoria": "Lattice Phantasmagoria.zip",
    "tabula-workbench": "Tabula Workbench.html",
    "lattice-deck": "Lattice Deck.html",
    "rag-workbench": "RAG Workbench.zip",
}
inventory = []
for slug, name in EXPORTS.items():
    original = DOWNLOADS / name
    raw = original.read_bytes()
    target = REPO / "examples" / "imports" / slug / "source"
    target.mkdir(parents=True, exist_ok=True)
    if original.suffix == ".zip":
        with zipfile.ZipFile(original) as archive:
            assert sum(info.file_size for info in archive.infolist()) < 32 << 20
            files = []
            for info in archive.infolist():
                if info.is_dir():
                    continue
                # Do not copy private customer/project notes into the public repo.
                if slug == "rag-workbench" and info.filename == "corpus.json":
                    continue
                path = target / info.filename
                assert path.resolve().is_relative_to(target.resolve()), info.filename
                assert not stat.S_ISLNK(info.external_attr >> 16), info.filename
                files.append((path, archive.read(info)))
    else:
        files = [(target / "index.html", raw)]
    for path, data in files:
        path.parent.mkdir(parents=True, exist_ok=True)
        if path.exists():
            assert path.read_bytes() == data, f"Source differs: {path}"
        else:
            path.write_bytes(data)
    inventory.append({
        "artifact": slug,
        "original": str(original),
        "sha256": hashlib.sha256(raw).hexdigest(),
        "bytes": len(raw),
        "files": [str(path.relative_to(REPO)) for path, _ in files],
    })
(TICKET / "sources").mkdir(exist_ok=True)
(TICKET / "sources" / "inventory.json").write_text(json.dumps(inventory, indent=2) + "\n")
print(json.dumps([{k: v for k, v in item.items() if k != "files"} for item in inventory], indent=2))
