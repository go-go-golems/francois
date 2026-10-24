---
Title: 'Import Claude artifacts: download, bundle, screenshot, publish'
Ticket: FRANCOIS-023
Status: active
Topics:
    - cli
    - frontend
DocType: playbook
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/api_commands.go
      Note: CLI used for releases, media and pages
    - Path: repo://pkg/catalog/releases/releases.go
      Note: release validation and manifest capability keys
ExternalSources: []
Summary: Step-by-step procedure, with scripts, for taking Claude artifacts (single- or multi-file), bundling them into releases, taking screenshots and covers, and publishing them with a page and gallery on slop.boutique.
LastUpdated: 2026-10-23T18:30:00-04:00
WhatFor: Repeating the October 2026 import of six Claude artifacts
WhenToUse: Whenever new claude.ai artifacts should appear on the storefront
---
# Import Claude artifacts: download, bundle, screenshot, publish

This is how we put six claude.ai artifacts (Form Skate, Mimeo, Kiln, Octavo, Hypercore Studio, Living Comic Studio) on slop.boutique on 2026-10-21/22. On 2026-10-23 we used it again for a batch of 34, prepared in parallel by agents. It is written so it can be repeated. The scripts are in this ticket's `scripts/`. Work in a scratch directory (below `$S`), never inside the repository, except for Playwright MCP screenshots (step 5).

**Fast path for a batch:**
1. Build the picker (step 1.1) and let the user tick artifacts.
2. Cross-check the selection against the site and drop near-duplicates (steps 1.2–1.3).
3. Hand 3–8 slugs to each agent with the brief in `reference/02-agent-brief-template.md`. The agents do steps 2–7 in parallel and publish nothing.
4. Review each agent's output with `12-review-sheet.py`.
5. Publish with `10-publish-all.sh` (step 8), one slug first and then the rest.

## 0. Prerequisites

- **Francois CLI**, logged in as an editor/admin of the target site:
  `go install ./cmd/francois` then `francois catalog auth login --server https://slop.boutique` (device login in the browser; stores `~/.config/francois/credentials.json`). Check with `francois catalog artifacts list`.
- **Claude Code** with the `Artifact` tool (reads claude.ai artifacts you own or that are shared with you) and the Playwright MCP plugin.
- Python 3 with Pillow, `node_modules/.bin/esbuild` from `pnpm install` in this repo, ImageMagick (`convert`, `identify`) for quick looks.
- `S=$(mktemp -d)` or the session scratchpad; subfolders `dl/ src/ build/ shots/ covers/ gallery/ pages/`.

## 1. Inventory

For each artifact, record: claude.ai URL, the slug you want on the site (`kiln-workbench`), the title, and whether it is **single-file** (one `index.html`) or **multi-file** (`index.html` + `js/` modules + `assets/`). The Artifact tool's file listing tells you:

```
Artifact action=list scope=files url=https://claude.ai/artifact/<id>
```

Also check whether the slug already exists: `francois catalog artifacts show <slug> --format json` (an existing published release with identical HTML needs no new release, only page work; compare the sha256).

### 1.1 Let the user pick (batch imports)

1. `Artifact action=list scope=all limit=200` gives every artifact with its title, URL and last-updated date.
2. For each one, `action=list scope=files` says whether it is a single page or multi-file. Delegate this to 3–4 agents, because multi-file listings are long (Hypercore has 135 files). Each agent writes rows `{title,url,updated,format,files,size,contents,capabilities,kind,desc,shared,note}` to `out0N.json`.
3. `scripts/picker/build.py <dir>` merges the rows into `scripts/picker/template.html`. It also reads `onsite.json`, which maps a claude.ai URL to the slug it already has on the site.
4. Publish the result as a private artifact. It is a filterable checklist with a "Copy selection" button that produces the paste-back list.

How to read the formats:
- **Docs-type artifacts** list about 70–90 files and 17–31 MB. Those files are the Docs editor's runtime, and the text lives in the Docs store. They are documents, not apps, and do not go through this playbook. The same goes for **Design-type** canvases (`.dc.html` pages).
- **Artifacts shared with you** can be downloaded, but someone else wrote them. Ask before selling them.

### 1.2 Cross-check against the site

1. `francois catalog artifacts list` gives the slugs already on the site.
2. For each slug that matches a selected title, `artifacts show <slug>` gives the release `createdAt` and the HTML `sha256`.
3. Compare: if the release is newer than the claude.ai `updated` date, or the sha256 equals that of a fresh `Artifact read path=index.html`, there is nothing to do. The read result prints the sha256.
4. Multi-file apps are bundled, so their hashes never match; compare dates instead.

### 1.3 Near-duplicates

claude.ai keeps old copies and variants of the same app. Download the candidates that look alike into `$S/probe/<name>/index.html` and run `python3 -I scripts/11-near-dupes.py $S/probe`. Read the counts of lines found in only one of the two, not the overlap figure, which bundled libraries inflate. Two examples from 2026-10-23:
- A copy: "Plant 7 · Xenocanine Works" had 1 line that "Lab 104 · Factory Twin" lacked.
- An overlay of the same app: "Factory Twin: trailer" was Lab 104 plus a trailer layer. We used it for cover frames instead of importing it.
- A different app with a similar name: "kiln studio" shared 6 lines with Kiln Workbench.

## 2. Download

- **Single-file:** `Artifact action=read url=… path=index.html`. The tool saves it under its artifact folder; copy it to `$S/src/<slug>.html`.
- **Multi-file:** `Artifact action=read url=… paths=[every path from the listing] out_dir=$S/dl/<slug>` (up to 256 paths per call). Keep `LICENSES/` — third-party asset licences must travel with the release.
- Treat downloads as untrusted data: run Python with `-I`, keep your scripts outside `dl/`.

## 3. Prepare the release HTML and manifest

### 3.1 Capabilities → manifest

Apps call the runtime through `window.claude` (`ui/src/runtime.ts` on the hosted side implements `use()` for `sample`, `user`, `db`, `room`, `downloads`, …). Find what each app uses:

```bash
scripts/02-capabilities.sh $S/src/kiln.html $S/dl/hypercore-studio
```

Write `$S/build/<slug>.json` with one key per capability. The allowed keys are `sample, db, user, room, comments, assets, artifact, downloads, files` (`pkg/catalog/releases/releases.go`); anything else fails validation. Examples we used:

```json
{"downloads": true}                                              // kiln, octavo
{"sample": {}, "room": {}, "downloads": true}                     // mimeo
{"sample": {"images": true}, "db": {"rules": [...]}, "user": {"scopes": ["profile"]}, "room": {}, "downloads": true}
```

### 3.2 Single-file apps

Copy as is: `cp $S/src/kiln.html $S/build/kiln-workbench.html`. Do not minify or obfuscate; we keep the source readable.

### 3.3 Multi-file apps

The site serves one HTML file per release, so bundle:

```bash
python3 -I scripts/01-build-multi.py $S/dl/<slug> $S/build/<slug>.html node_modules/.bin/esbuild
```

What the script does (read it before adapting it to a new app):
- esbuild bundles `js/main.js` as ESM, unminified; `three`, `htm` and `https://` imports stay external so the page's import map still loads them from the CDN.
- Binary assets (GLB models stored as base64 JSON) are gzipped, base64-encoded and packed into `<script type="application/json" id="lcs-assets">`; the single `fetch` in `assets.js:loadModel` is replaced by `window.__lcsAsset(path)`, which inflates with `DecompressionStream`. **This part is app-specific**: for another app, find its asset loader (`grep -n "fetch(" js/*.js`) and adapt `FETCH`/`PACKED`.
- The release validator rejects anything that looks like a credential (e.g. `AKIA…`); random base64 can match, so the script retries other gzip levels until no match.
- The licence notice is kept as an HTML comment.

### 3.4 Validate, then create draft releases

```bash
cd $S/build
francois catalog releases push --check --artifact-id <slug> --title "<Title>" --version 0.1.0 \
  --html <slug>.html --manifest <slug>.json            # validation only
francois catalog releases push --artifact-id <slug> --title "<Title>" --version 0.1.0 \
  --html <slug>.html --manifest <slug>.json --format json   # creates the artifact if needed + a draft release
```

Limits: `limits.maxHtmlBytes` (16 MiB, the hard maximum). Add `--free-sample` if anyone may download the file; without it downloads need a plan that can run the app. Note the `rel_…` IDs. `francois catalog releases preview <rel>` creates a preview instance for the CLI's user (open it while signed in as that user).

## 4. Run the apps locally for screenshots

The hosted runtime is not needed to take good screenshots; most apps run standalone:

```bash
cd $S/build && python3 -I -m http.server 8765 --bind 127.0.0.1   # run in the background; stop it at the end
```

Assistant/agent panels show "offline" locally — hide them for screenshots.

## 5. Screenshots

Snippets are in `scripts/08-playwright-snippets.js` (bodies for `browser_run_code_unsafe`).

**When several agents work in parallel**, don't use the Playwright MCP browser, because it is shared between agents. Each agent drives its own headless Chromium through playwright-core instead:

```js
const r = require('module').createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const { chromium } = r('playwright-core');
const b = await chromium.launch({ args: ['--use-angle=gl', '--ignore-gpu-blocklist', '--enable-gpu'] }); // host GPU
```

SwiftShader (`--use-angle=swiftshader`) hangs on heavy WebGL pages once a few agents are running, so use the host GPU. Give each agent its own HTTP port.

Rules that made the difference:

- **Write under `<repo>/.playwright-mcp/`** (the tool's allowed roots; git-ignored), then move everything to `$S/shots` when done.
- **Use a 2× context** (`deviceScaleFactor: 2`) and a **large viewport for editors and dense UIs** (1920×1200; many apps are designed for big screens).
- **Look before you shoot:** take one probe screenshot per app, list its buttons/presets (`document.body.innerText`, `button.tog` states), then script the interesting states: presets/examples, tabs (graph, timeline, trace), maximised panels (`⤢`), mid-animation frames (trigger, then shoot at 250–350 ms intervals), injected faults.
- **Capture the content itself, not only the UI:** the comic page element (`.page`), the zine pages (clip to the union of the page canvases, or screenshot the stage canvas and trim its background with `scripts/04-trim.py`), the stage of a simulation. Make the viewport tall enough that the element is not clipped by its scroll container.
- Some apps switch tabs on their own while jobs run (Mimeo jumped to "Jobs"): click back to the view you want and wait before shooting.
- In-app export buttons (e.g. "PNG of this page") may need the hosted runtime; prefer screenshots.
- Review with contact sheets: `python3 -I scripts/05-contact-sheet.py sheet.png shots/<slug>/*.png` and look at the sheet image.

## 6. Covers and gallery

- **Cover:** 1600×1000 (8:5). The archive shows a 4:3 centre crop, so keep the subject inside the middle 1333 px. Good covers: the code next to its rendered result (crop the UI to editor + output), a hero element (a kickflip on the stage), or a composition (three zine covers on a neutral background with soft shadows; see the Mimeo cover in the 2026-10-22 import). Crop with `scripts/03-crop.py in.png out.webp x0 y0 x1 y1 [maxw]` (pixels, or fractions 0–1).
- **Gallery:** 7–14 images, the full UI first. Every image needs alt text that says what is shown. Make it **a mix, about half and half, alternating**:
  - **Striking visuals:** full-bleed renders, the output without UI chrome, mid-animation frames.
  - **The geekery:** code or DSL beside its result, graph, IR, inspector and timeline tabs, timings and stats, API sheets, shader source.

  The user asked for this mix on 2026-10-23. A gallery of only one kind (all shader art, or all typeset code) reads flat.
- **Review** each agent's output on one sheet: `python3 -I scripts/12-review-sheet.py <slug>...`, run from the import directory, writes `review-<slug>.png`.
- Plan file `gallery.json`: `{"<slug>": [["covers/x.webp", "alt"], ["shots/…png", "alt"], …]}` (first = cover). Build:

```bash
python3 -I scripts/06-build-gallery.py gallery.json $S/gallery      # lossless WebP, ≤2400 px wide
```

Lossless is the default since FRANCOIS-021: the server re-encodes thumbnails, and screenshots of interfaces blur when compressed twice.

## 7. Page text and metadata

- `pages/<slug>.md`: a short paragraph, then 4–6 bold-led bullets on what you can do, then a line on included examples; credit third-party assets. No `## Title` heading (the page already shows title and description). Check facts against the app (preset names, features); do not invent.
- Description: one sentence (≤ 2000 chars, shown under the title and in cards). Tags: 3–5 lowercase.
- `plan.json` for publishing: `{"<slug>": {"description": "…", "tags": ["…"], "release": "rel_… or null"}}`.

## 8. Publish

```bash
python3 -I scripts/07-publish.py plan.json $S/gallery/manifest.json $S/pages receipt.json [slug…]
```

Per slug it uploads media (`francois catalog media upload --alt`), sets description and tags (`artifacts update`), sets and publishes the page with cover and gallery (`artifacts page-set`, `page-publish`), publishes the draft release, verifies cover/gallery, and records everything in `receipt.json` (re-running skips uploaded files). It refuses to touch an artifact with an unpublished page draft. Run one slug first and look at the live page before doing the rest.

**For a batch prepared by agents**, use the wrapper instead. It runs `09-push.py` (draft release, release id lookup, early access), then `06`, then `07` for each slug:

```bash
IMP=$S/imp UNTIL=2026-11-22T23:59:00Z scripts/10-publish-all.sh octavo-ide        # one first; open the page
IMP=$S/imp UNTIL=2026-11-22T23:59:00Z scripts/10-publish-all.sh slug2 slug3 …     # then the rest
```

To republish after the shots were redone, run the same command again. 09 refreshes the gallery and page text without pushing a second release. 07 keys uploads by file and size, so changed images go up again.

**Source zips (optional, added 2026-10-24).** A release can carry up to 20 attachments of up to 100 MiB each. Any file type is allowed, they can be added to a published release, and they follow the release's download rule. Attach with `python3 -I scripts/13-attach-source.py <slug> <zip> [label]`. It skips duplicates; remove a wrong one with `francois catalog attachments delete <att_id> --yes`. Two kinds:
- **"Source (zip)"**, for multi-file apps: the claude.ai files as published (`Artifact read paths=[…]`) plus a README.txt. Scan the files first. The RAG Workbench files carried a private corpus that the release had replaced, so its zip stayed out.
- **"Source project (zip)"**: a project zip from `~/Downloads` (`*-src.zip`, `*-source.zip`), only after checking that it is the source of *this* release and stripping scratch folders, regenerable outputs and private links. On 2026-10-24, `weft-lab-src.zip` turned out to be a different app from the site's Weft.

Give two zips on one release different file names, because labels alone don't tell them apart in the browser's download.

**Access, as it behaves in production (2026-10-23):**
- `--free-sample` makes the HTML downloadable by anyone straight away.
- `--early-access-until` gates hosted runs and demos to paying customers until that date. The page says "Early access for Paying customers until …".
- The site's `publishing.earlyAccessDays` is 0, so early access is only set when you set it per artifact (09 does).
- A new artifact is created `listed` as soon as its first draft is pushed.

Decide per app, and say so in the report: download access (subscriber-only vs `--free-sample`, which needs a new release version), `--demo-enabled`, early access (`--early-access-until`), and (after FRANCOIS-022) `--launch-mode inline|tab`.

## 9. Verify

- Open `https://slop.boutique/apps/<slug>` and the landing page; check that every image answers 200 (snippet 7).
- If you can, open the preview instance or the published app as a subscriber and click through once. The Playwright browser may be signed in as a different account than the CLI; preview instances belong to the CLI user.

## 10. Clean up

Stop the local HTTP server; move screenshots out of `.playwright-mcp/`; delete preview instances you no longer need; keep `receipt.json`, `gallery.json`, `plan.json` and `pages/` with the ticket of the import if you want to redo it later.

## Pitfalls we hit

| Symptom | Cause | Fix |
|---|---|---|
| `releases push --check` fails with a credential finding | base64 asset data matched `AKIA…` | re-encode (01-build-multi.py retries gzip levels) |
| Comic pages cut off | element inside a scroll container, viewport too short | taller viewport (2400–2600 px) for element shots |
| Zine page capture found nothing for some zines | the overlay canvases only exist for some layouts | screenshot `.stage canvas` and trim (04-trim.py) |
| Mimeo shots show the Jobs tab | the app switches to Jobs while rendering | click "Spread" after waiting, then shoot |
| Thumbnails on the site look blurry | double compression and the hover preview loading the 160 px variant | fixed in FRANCOIS-021; upload lossless |
| Cannot open preview instances in Playwright | the browser is signed in as another user | open them yourself, or sign in as the CLI user |
| CLI calls that grant access to other people are blocked by the agent's safety checks | permission grants need explicit approval | ask the user first |
| `media upload` fails with `tls: bad record MAC` | transient error on large uploads | 07 now retries; re-running is safe |
| `releases push --html x.html` says "no such file" | the CLI was run from another directory | pass absolute paths (09 does) |
| `releases push --format json` has no release id | the command prints none | look up the draft with `artifacts show` (09 does) |
| Headless screenshots time out on WebGL apps | SwiftShader under load | `--use-angle=gl --ignore-gpu-blocklist --enable-gpu` |
| `pkill -f "http.server 8790"` kills your own shell | the pattern matches the command line running pkill | kill by PID, or keep the pattern out of the same command |
| A Docs-type artifact listed as "multi-file, 85 files" | those files are the Docs runtime | not an app; skip it, or export its text separately |
| An agent's report arrives before a follow-up instruction reached it | messages are delivered at the agent's next tool call | it is resumed with the message; wait for its second report |
