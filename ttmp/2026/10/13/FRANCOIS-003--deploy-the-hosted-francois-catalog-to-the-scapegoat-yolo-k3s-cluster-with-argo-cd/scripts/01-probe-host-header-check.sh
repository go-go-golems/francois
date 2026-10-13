#!/usr/bin/env bash
# Shows how `francois catalog serve` answers a kubelet-style probe (Host =
# pod IP:port) versus a probe that sends the portal Host header. Uses the
# local Compose PostgreSQL (127.0.0.1:5435) and the fake model backend.
set -euo pipefail
BIN=${BIN:-$(mktemp -d)/francois}
go build -o "$BIN" ./cmd/francois
"$BIN" catalog serve --listen 127.0.0.1:18899 \
  --public-url https://francois.yolo.scapegoat.dev \
  --app-domain apps.francois.yolo.scapegoat.dev \
  --database-url "postgres://francois:francois@127.0.0.1:5435/francois_catalog?sslmode=disable" \
  --release-dir "$(mktemp -d)" --worker-dir "$(mktemp -d)" --backend fake >/dev/null 2>&1 &
pid=$!
trap 'kill $pid' EXIT
for _ in $(seq 1 50); do curl -s -o /dev/null 127.0.0.1:18899/ && break; sleep 0.2; done
probe() { printf '%-55s -> ' "$1"; curl -s -o /dev/null -w '%{http_code}\n' "${@:2}"; }
probe "GET /healthz, Host 10.42.0.17:8080 (kubelet default)" -H 'Host: 10.42.0.17:8080' http://127.0.0.1:18899/healthz
probe "GET /readyz,  Host 10.42.0.17:8080 (kubelet default)" -H 'Host: 10.42.0.17:8080' http://127.0.0.1:18899/readyz
probe "GET /healthz, Host francois.yolo.scapegoat.dev"       -H 'Host: francois.yolo.scapegoat.dev' http://127.0.0.1:18899/healthz
probe "GET /readyz,  Host francois.yolo.scapegoat.dev"       -H 'Host: francois.yolo.scapegoat.dev' http://127.0.0.1:18899/readyz
probe "GET /,        Host i-unknown.apps.francois.yolo.scapegoat.dev" -H 'Host: i-unknown.apps.francois.yolo.scapegoat.dev' http://127.0.0.1:18899/
probe "GET /,        Host a.b.apps.francois.yolo.scapegoat.dev" -H 'Host: a.b.apps.francois.yolo.scapegoat.dev' http://127.0.0.1:18899/
