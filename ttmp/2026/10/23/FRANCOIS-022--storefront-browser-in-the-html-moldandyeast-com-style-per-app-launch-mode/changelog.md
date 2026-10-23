# Changelog

## 2026-10-23

- Initial workspace created

## 2026-10-23

Studied html.moldandyeast.com and the storefront; wrote the design and implementation guide (browser, launch modes, site copy, early-access subscribe message, accent token).

## 2026-10-23

Phases 1–4 implemented (d8855d3 site copy, 1094837 launch mode, cbbe3df landing browser, 1055630 client viewer/tab launch/subscribe invite); validated with go test, pnpm check/test and Playwright on a local preview

### Related Files

- /home/manuel/code/wesen/go-go-golems/francois/pkg/catalog/pages.go — Feed and app page data
- /home/manuel/code/wesen/go-go-golems/francois/ui/catalog/launch-decision.ts — Launch rules
- /home/manuel/code/wesen/go-go-golems/francois/ui/catalog/public.ts — Browser, viewer and runner client

## 2026-10-23

Pre-deploy check: all six real apps run in the inline viewer (Live in ~1.1 s, Desktop/Tablet/Phone) and in a new tab; removed redundant allowfullscreen (f0d389e). Comic apps overflow at Phone width (app layout).

### Related Files

- /home/manuel/code/wesen/go-go-golems/francois/ui/catalog/public.ts — mountFrame iframe attributes
