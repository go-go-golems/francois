#!/usr/bin/env bash
# Copies the k3s repo's gitops/ tree and validator into a scratch directory,
# adds this ticket's draft francois package, then runs the repository's own
# scripts/validate_gitops.sh (render + PVC/workload sync-wave invariant +
# no inline private keys). No cluster access.
set -euo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
k3s=${K3S_REPO:-/home/manuel/code/wesen/2026-03-27--hetzner-k3s}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/scripts"
cp -r "$k3s/gitops" "$work/"
cp "$k3s/scripts/validate_gitops.sh" "$work/scripts/"
cp -r "$here/reference/gitops-draft/gitops/kustomize/francois" "$work/gitops/kustomize/"
cp "$here/reference/gitops-draft/gitops/applications/francois.yaml" "$work/gitops/applications/"
cd "$work"
bash scripts/validate_gitops.sh 2>&1 | grep -iE 'francois|violation|pass|fail|ok' | grep -iE "francois|Violations|RESULT"
echo "exit=${PIPESTATUS[0]}"
