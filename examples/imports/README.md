# Imported HTML proofs of concept

These applications come from the named Downloads exports in ticket FRANCOIS-009.
Original program sources are retained under each application's `source/`; the
builder makes `release.html` and its explicit hosted capability manifest.

| Application | Runtime capabilities |
| --- | --- |
| Lattice Rift | Browser-only WebGL interaction |
| Lattice Phantasmagoria | Browser-only WebGL/canvas interaction |
| Tabula Workbench | Model sampling and downloads; browser-local workbook state |
| Lattice Deck | Model sampling, documents, user profile and rooms |
| RAG Workbench | Model sampling, documents, user profile and downloads |

Build the single-file releases from the repository root:

```sh
python3 ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/scripts/02-build-imports.py
```

The copy script in that directory documents the original Downloads filenames
and verifies their SHA256 inventory. RAG's original private corpus is deliberately
excluded. Its `demo-corpus.json` contains public sample documentation, embedded
along with the preset library into the release. The original Downloads ZIP is
never modified.

The builder preserves the visual design and makes only explicit hosting edits:
local script/resource embedding, Deck's fire-and-forget presence handling,
RAG's initial lexical preset/query and its default single-call map concurrency.
Pinned external CDN scripts and Google Fonts links remain as in the exports.
See the ticket's guide, diary and browser evidence for validation/publication.
