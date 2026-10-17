#!/usr/bin/env bash
set -euo pipefail
ticket_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for diagram_name in 01-catalog-read-sequence 02-shared-catalog-access; do
  dot -Tpng "$ticket_dir/sources/$diagram_name.dot" -o "$ticket_dir/sources/$diagram_name.png"
done
