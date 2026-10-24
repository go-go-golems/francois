# Changelog

## 2026-10-23

- Initial workspace created

## 2026-10-23

Playbook and scripts for importing Claude artifacts (from the 2026-10-21/22 import of six apps).

## 2026-10-23

- Playbook revised after a 34-app batch import: an artifact picker (`scripts/picker/`), cross-checking against the site and near-duplicate detection (`11-near-dupes.py`), parallel preparation agents with a reusable brief (`reference/02-agent-brief-template.md`), headless Chromium with the host GPU, the visuals/geekery gallery mix, review sheets (`12-review-sheet.py`), and publishing with `09-push.py` and `10-publish-all.sh`. `07-publish.py` now retries transient upload errors and re-uploads images that changed.
