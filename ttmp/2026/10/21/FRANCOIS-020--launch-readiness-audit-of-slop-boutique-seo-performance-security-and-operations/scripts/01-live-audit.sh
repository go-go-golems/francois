#!/usr/bin/env bash
# Re-runs the live checks behind the FRANCOIS-020 launch readiness audit.
# Usage: scripts/01-live-audit.sh [origin]   (default https://slop.boutique)
set -u
O=${1:-https://slop.boutique}
H=${O#https://}
UA_BROWSER='Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36'
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

echo "## Transport"
printf 'http://%s/  -> ' "$H"; curl -sS -o /dev/null -w '%{http_code} %{redirect_url}\n' "http://$H/"
printf 'www          -> '; curl -sS -o /dev/null -w '%{http_code} %{redirect_url}\n' "https://www.$H/"
printf 'HSTS         -> '; curl -sS -o /dev/null -D - "$O/" | grep -i strict-transport-security || echo missing

echo "## Crawl files and errors (code content-type)"
for p in /robots.txt /sitemap.xml /favicon.ico /apple-touch-icon.png /.well-known/security.txt /feed.xml /no-such-page /apps/no-such-app; do
  printf '%-28s ' "$p"; curl -sS -o /dev/null -w '%{http_code} %{content_type}\n' "$O$p"
done

echo "## Head tags per page"
for p in / /apps /apps/form-skate-workbench /apps/rag-workbench /pricing /privacy /terms /signin /studio; do
  curl -sS -D "$tmp.h" -o "$tmp" "$O$p"
  printf '%-28s title=%s | canon=%s | og:image=%s | robots=%s\n' "$p" \
    "$(grep -o '<title>[^<]*' "$tmp" | head -1 | cut -c8-)" \
    "$(grep -o 'rel="canonical" href="[^"]*' "$tmp" | cut -d'"' -f4)" \
    "$(grep -c 'og:image' "$tmp")" \
    "$(grep -i x-robots-tag "$tmp.h" | tr -d '\r' | cut -d' ' -f2-)"
done
rm -f "$tmp.h"

echo "## Caching and compression"
printf 'HTML (crawler UA) : '; curl -sS -o /dev/null -D - -H 'Accept-Encoding: gzip' "$O/" | grep -i -E '^(cache-control|content-encoding)' | tr -d '\r' | tr '\n' ' '; echo
printf 'HTML (browser UA) : '; curl -sS -o /dev/null -D - -A "$UA_BROWSER" -H 'Accept-Encoding: gzip' "$O/" | grep -i -E '^(cache-control|content-encoding)' | tr -d '\r' | tr '\n' ' '; echo
for u in /fonts/fonts.css /fonts/ABCAreal-Regular.woff2; do
  printf '%-18s: ' "$u"; curl -sS -o /dev/null -D - "$O$u" | grep -i '^cache-control' | tr -d '\r'
done

echo "## Homepage image bytes (displayed at 76x54 CSS px)"
curl -sS "$O/" | grep -o 'src="/media/[^"]*' | cut -d'"' -f2 | while read -r m; do
  printf '%-36s %8s bytes\n' "$m" "$(curl -sS -o /dev/null -w '%{size_download}' "$O$m")"
done

echo "## DNS (public resolver)"
for q in "A $H" "AAAA $H" "CAA $H" "TXT $H" "TXT _dmarc.$H" "CNAME _dmarc.$H"; do
  printf '%-26s ' "$q"; dig +short @1.1.1.1 $q | tr '\n' ' '; echo
done

echo "## TTFB x3"
for i in 1 2 3; do curl -sS -o /dev/null -w 'connect=%{time_connect} ttfb=%{time_starttransfer}\n' "$O/"; done
