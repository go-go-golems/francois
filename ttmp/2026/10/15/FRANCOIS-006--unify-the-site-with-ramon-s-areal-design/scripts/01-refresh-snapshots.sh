#!/usr/bin/env bash
# Copies Paper board exports (Paper writes them to ~/Downloads, named after the
# board, with " (1)" etc. for repeats) over the ticket snapshots and rebuilds
# the overview sheets. Usage: 01-refresh-snapshots.sh [export-dir]
set -euo pipefail

src=${1:-$HOME/Downloads}
here=$(cd "$(dirname "$0")/.." && pwd)
after=$here/sources/design-snapshot-2026-10-15-after
states=$here/sources/design-snapshot-2026-10-15-states-mobile

# newest export whose name starts with the board's code and a space
newest() {
	local f
	f=$(ls -t "$src/$1 "*.jpg 2>/dev/null | grep -v '@2x' | head -1) || true
	[[ -n $f ]] || { echo "no export for board $1 in $src" >&2; exit 1; }
	printf '%s' "$f"
}

declare -A boards=(
	[S1]=$after/S1-home.jpg [S2]=$after/S2-artifact.jpg [S3]=$after/S3-pricing.jpg
	[S4]=$after/S4-journal-post.jpg [S5]=$after/S5-creator.jpg
	[A1]=$after/A1-account.jpg [A2]=$after/A2-instances.jpg [A3]=$after/A3-demo-running.jpg
	[T1]=$after/T1-studio-artifacts.jpg [T2]=$after/T2-studio-page-editor.jpg
	[T3]=$after/T3-studio-releases-attachments.jpg [T4]=$after/T4-studio-media.jpg
	[D1]=$after/D1-design-system.jpg
	[S6]=$states/S6-catalog.jpg [S7]=$states/S7-journal.jpg
	[S2b]=$states/S2b-artifact-subscriber.jpg [S2c]=$states/S2c-artifact-plan-required.jpg
	[S2d]=$states/S2d-artifact-draft-preview.jpg
	[A4]=$states/A4-instance-running.jpg [A5]=$states/A5-instance-people.jpg
	[A6]=$states/A6-instance-paused.jpg [A7]=$states/A7-instance-frozen.jpg
	[A8]=$states/A8-demo-ended.jpg [A9]=$states/A9-demo-full.jpg
	[A10]=$states/A10-checkout-confirming.jpg [A11]=$states/A11-checkout-welcome.jpg
	[A12]=$states/A12-checkout-cancelled.jpg
	[M1]=$states/M1-home-390.jpg [M2]=$states/M2-artifact-390.jpg
)
for code in "${!boards[@]}"; do
	cp "$(newest "$code")" "${boards[$code]}"
done

# Rows of boards at a common width, top-aligned on Paper's canvas grey.
sheet() {
	local out=$1 width=$2
	shift 2
	montage "$@" -resize "${width}x" -background '#E6E6E6' -gravity North \
		-geometry "+12+12" -tile "${TILE}" -quality 85 "$out"
}
TILE=7x sheet "$after/overview.jpg" 420 \
	"$after"/S{1,2,3,4,5}-*.jpg "$after"/A{1,2}-*.jpg "$after"/A3-*.jpg \
	"$after"/T{1,2,3,4}-*.jpg "$after"/D1-*.jpg
TILE=6x sheet "$states/overview-desktop.jpg" 420 \
	"$states"/S6-*.jpg "$states"/S7-*.jpg "$states"/S2{b,c,d}-*.jpg "$states"/A4-*.jpg \
	"$states"/A5-*.jpg "$states"/A6-*.jpg "$states"/A7-*.jpg "$states"/A8-*.jpg "$states"/A9-*.jpg \
	"$states"/A10-*.jpg "$states"/A11-*.jpg "$states"/A12-*.jpg
TILE=2x sheet "$states/overview-mobile.jpg" 390 "$states"/M1-*.jpg "$states"/M2-*.jpg
