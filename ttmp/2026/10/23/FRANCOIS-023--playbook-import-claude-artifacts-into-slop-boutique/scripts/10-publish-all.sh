#!/bin/sh
# usage: IMP=<imp-dir> UNTIL=<RFC3339|-> 10-publish-all.sh slug...
# Push draft releases (09), build lossless gallery WebPs (06), publish page + release (07)
# one slug at a time, then print the live page status. 07 retries transient upload errors.
set -u
S=$(cd "$(dirname "$0")" && pwd); P=$IMP/pub
python3 -I "$S/09-push.py" "$IMP" "$UNTIL" "$@" || exit 1
python3 -I "$S/06-build-gallery.py" "$P/gallery.json" "$P/gallery" >/dev/null || exit 1
for s in "$@"; do
  python3 -I "$S/07-publish.py" "$P/plan.json" "$P/gallery/manifest.json" "$P/pages" "$P/receipt.json" "$s" > "$P/log-$s.txt" 2>&1
  printf '%s: %s / page %s\n' "$s" "$(tail -1 "$P/log-$s.txt")" "$(curl -s -o /dev/null -w '%{http_code}' "https://slop.boutique/apps/$s")"
done
