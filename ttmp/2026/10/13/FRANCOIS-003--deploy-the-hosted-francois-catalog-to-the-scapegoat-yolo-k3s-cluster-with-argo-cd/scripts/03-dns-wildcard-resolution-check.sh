#!/usr/bin/env bash
# Asks the authoritative DigitalOcean name server how the scapegoat.dev zone
# answers the proposed Francois names. DigitalOcean synthesizes *.yolo for
# multi-label names and for names below other wildcard records.
set -euo pipefail
for n in francois.yolo.scapegoat.dev i-abc.apps.francois.yolo.scapegoat.dev \
         hosting.yolo.scapegoat.dev x.hosting.yolo.scapegoat.dev a.b.yolo.scapegoat.dev; do
  printf '%-42s %s\n' "$n" "$(dig +short A "$n" @ns1.digitalocean.com | tr '\n' ' ')"
done
