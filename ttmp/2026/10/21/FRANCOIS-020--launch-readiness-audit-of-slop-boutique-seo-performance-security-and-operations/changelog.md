# Changelog

## 2026-10-21

- Initial workspace created

## 2026-10-21

Audited live slop.boutique (curl, dig, Lighthouse 12.8.2) and wrote the launch readiness findings and checklist; added scripts/01-live-audit.sh to re-run the live checks

### Related Files

- ttmp/2026/10/21/FRANCOIS-020--launch-readiness-audit-of-slop-boutique-seo-performance-security-and-operations/analysis/01-launch-readiness-audit-findings-and-checklist.md — Findings and prioritized checklist

## 2026-10-21

Design doc 01 and implementation on feat/launch-readiness: --redirect-host retires francois.yolo.scapegoat.dev (308 to primary), HTTPS upgrade + HSTS, robots.txt/sitemap.xml, noindex on SPA and instance hosts, catch-all HTML 404 and 503 page, icons and manifest, og:type/twitter card, versioned+preloaded fonts, feed self links, --contact-email, a11y fixes. Hostnames only come from flags.

### Related Files

- /Users/manuel.odendahl/code/go-go-golems/francois-launch/doc/production.md — New flags and host retirement procedure
- /Users/manuel.odendahl/code/go-go-golems/francois-launch/pkg/catalog/seo.go — Crawl files
