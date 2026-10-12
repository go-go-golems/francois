---
Title: Investigation diary
Ticket: FRANCOIS-001
Status: active
Topics:
    - backend
    - llm
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-11T18:38:50.303137865-04:00
WhatFor: ""
WhenToUse: ""
---

# Investigation diary

## Step 1: Capture scope, source evidence, and template baseline

The request combines repository initialization with a technical research handoff. I initialized the empty application repository and investigated the downloaded runtime descriptions and POLIS source. Service implementation is reserved for the ticket's open tasks.

### Prompt Context

**User prompt (verbatim, interrupted first message):**

> Initialize this repo to be using go-go-golems/go-template, then look at the last 4 md I downloaded in ~/Downloads describing the cluade artifact APIs, Look at ~/Dow

**User prompt (verbatim, continuation):**

> POLIS IDE.html in Downloads too.
>
> Creat ea new docmgr ticket to implement glazed command go hosting service that can serve these html files and provide the claude APIs to make them work, except we are going to use ~/.pi/agent pi agent config files to manage the LLM side of things (and load its auth settings and models and such).
>
> Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.
>
> The analysis is detailed, the design is simple, straightforward, elegant, composable, no nonsense, no over cautious design, no over engineering.

**Assistant interpretation:** Initialize the scaffold, create an implementation ticket, write a detailed guide grounded in the supplied files and installed Pi APIs, and upload the guide. Do not implement the hosting service in this research turn.

### What I did

- Created FRANCOIS-001 with docmgr and added a design document and this diary.
- Sorted Downloads Markdown files by modification time and copied the newest four plus `Polis IDE.html`; recorded exact hashes in `sources/manifest.json`.
- Copied tracked template files from local commit `5bf14bfd4a7b13124dbe9f2c3078c29a33627a11`; replaced XXX placeholders with francois while preserving existing Git and docmgr files.
- Added Cobra/Glazed root logging, help, version, and signal cancellation. No serve command was added.
- Read supplied contracts, POLIS browser tools, installed Pi 0.85.1 SDK/types, and sanitized configuration structure. No credentials were copied into the ticket.
- Wrote the 17-section intern guide, including concrete protocols, diagrams, pseudocode, package map, and phase acceptance criteria.

### Why

POLIS requires browser-owned eval tools, not just a model completion proxy. Pi ModelRuntime already owns provider auth and custom model composition, so a small TypeScript worker avoids duplicating OAuth logic in Go. Go remains the owner of HTTP, view lifecycle, persistence, and event routing.

### What worked

- Exact evidence was found, copied, and hashed.
- `GOCACHE=/tmp/francois-go-cache GOWORK=off go test ./...` passed on the initialized CLI.
- Build, generation, `make logcopter-check`, and `go vet ./...` passed with the same temporary Go build cache.
- `make lint` passed with zero issues using the installed golangci-lint 2.4.0 and `/tmp/francois-lint-cache`. The template version file pins 2.12.2; that exact version was not installed/run in this turn.
- `go run ./cmd/francois --help` displayed Glazed logging flags; `--version` returned `francois version dev`.

### What didn't work

- The initial glob used uppercase `*POLIS*`; zsh reported `no matches found: /home/manuel/Downloads/*POLIS*`. A sorted directory listing found the actual mixed-case filename.
- Initial `GOWORK=off go test ./...` tried to write the restricted home build cache and failed with `open /home/manuel/.cache/go-build/...: read-only file system`. Recovery used `GOCACHE=/tmp/francois-go-cache`.
- `git ls-remote https://github.com/go-go-golems/go-template.git HEAD` failed with `ssh: Could not resolve hostname github.com: Name or service not known`. The existing local template checkout provided an exact revision; online browsing confirmed repository provenance but was not used to claim a refreshed Git checkout.
- A later Go build printed `go: writing stat cache: mkdir /home/manuel/go/pkg/mod/cache/download/github.com/go-go-golems/francois: read-only file system` but exited successfully. No module-cache permissions were changed.

### What I learned

The installed Pi SDK uses ModelRuntime, including auth-aware model discovery and streamSimple. Its settings default can differ from enabled-model scope. The supplied guides leave some permissions and files-form signatures underspecified; the guide makes those explicit implementation gates rather than inventing APIs.

### What was tricky to build

The important boundary is model tools that must execute in the original browser tab. Cancellation, call IDs, completed tool arguments, and continuation messages need to survive browser/Go/Node routing. Another subtlety is that URL paths share localStorage; independent artifact origins are required before claiming storage isolation.

### What warrants a second pair of eyes

Review model selection when configured default is outside enabled scope, browser tool result ownership, snapshots after reconnect, and per-artifact origin isolation. Review the original runtime types before implementing permissions and files-form publish. The inherited release files remain template infrastructure and do not constitute configured production publishing.

### What should be done in the future

Implement the open phase tasks in order; demonstrate the complete POLIS eval round trip before adding unrelated shell features. Do not close implementation tasks based on this guide's delivery.

### Code review instructions

Start with `cmd/francois/main.go`, README, and the guide's source analysis and Pi integration sections. Check the source manifest against its copied files. Run build/tests/lint/generation with a writable build cache if home caches remain restricted.

### Technical details

The proposed process topology is Go HTTP + one Node/Pi worker over NDJSON. The page runtime uses POST commands and one SSE event stream per view. SQLite owns durable document/version data; rooms remain memory-only. Details and precise source references live in the design document rather than being duplicated here.

## Step 2: Validate and deliver the research handoff

Repository validation completed and docmgr doctor reported all checks passed. The source snapshots matched all five recorded hashes and the guide had balanced code fences with no remaining template placeholders. The service remains unimplemented, with phases retained as open tasks.

The first upload failed on sandbox DNS (`lookup webapp-prod.cloud.remarkable.engineering: no such host`). Retrying the same authorized upload with escalated network access succeeded. No manual reauthentication or overwrite was required.

### Delivery receipt

- Command: `remarquee upload bundle <guide> --name 'FRANCOIS-001 Artifact Host Intern Guide' --remote-dir '/ai/2026/10/11/FRANCOIS-001' --toc-depth 2 --non-interactive`.
- Result: `OK: uploaded FRANCOIS-001 Artifact Host Intern Guide.pdf -> /ai/2026/10/11/FRANCOIS-001`.
- This proves cloud delivery, not physical device synchronization.
- Validation: Go tests/build/generate/vet, logcopter check, CLI help/version, installed-tool lint, source hashes, docmgr doctor.
- No hosting server or paid provider call was run. No Git commit, push, release, or remote creation was performed.

## Step 3: Begin implementation and establish the styling reference

The user authorized step-by-step implementation, milestone commits, and a detailed investigation diary. The implementation starts from the completed scaffold and design. The downloaded Factory Twin trailer supplies the visual language for Francois-owned surfaces; imported applications retain their original CSS.

### Prompt Context

**User prompt (verbatim):**

> Use a clean minimalist look like in ~/Downloads/factory-twin-trailer.html for the CSS.
>
> Implement step by step. commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)

**Assistant interpretation:** Implement the ticket incrementally, use the provided visual reference, validate each meaningful milestone, and commit changes with the investigation diary.
**Inferred user intent:** Deliver a working local artifact host with clear implementation evidence and reviewable history.

### What I did

- Read the current ticket tasks, repository guidelines, Glazed APIs, diary format, and commit hygiene instructions.
- Inspected Factory Twin CSS tokens: gray #f6f7f9, white panels, #e3e6ea borders, ink #1f2630, blue #2563eb, compact 8–10 px radii, Instrument Sans and JetBrains Mono fallbacks.
- Prepared a baseline commit of the already validated scaffold and research handoff.

### Why

A baseline commit separates the earlier design deliverable from the functional changes and preserves the exact original inputs.

### What worked

The requested reference exists and has a complete standalone HTML application. The scaffold validation from Step 1 remains applicable before implementation edits.

### What didn't work

No new failures in this step.

### What I learned

The reference is an application UI with light panels and restrained accents. It does not require copying its canvas/scene renderer into Francois.

### What was tricky to build

The template AGENT.md contains stale process instructions and a placeholder-replaced port command. The user's explicit tmux and lsof-who instructions take precedence.

### What warrants a second pair of eyes

Review the browser tool boundary and model/auth behavior before treating the host as a general shared service.

### What should be done in the future

Build the serve/import/model command path, embedded UI, browser tool transport, and Pi worker first, then persistent capabilities.

### Code review instructions

Use the baseline commit for comparison. Subsequent diary entries describe actual changes and verification.

### Technical details

Go owns HTTP/storage; TypeScript owns the Pi worker and browser runtime; React/Redux/RTK Query own the host UI. All static resources are served under /static/.

## Step 4: Implement the local host and prove POLIS browser tools

The repository now has a functioning artifact library, isolated artifact origins, persistent imports/versions, view-owned SSE calls, a pinned Pi worker, and an embedded React UI styled from Factory Twin. A browser experiment opened the actual POLIS source and executed its eval callback through Go, saving a new preset before receiving the final model result.

### Prompt Context

See Step 3 for the implementation request. The user also asked: “how could we map the claude sample API to allow the richer model selection, or would that already be possible?” Existing `sample(...,{model,effort})` supports Pi provider/model selectors; tier flags remain the fallback. `/api/models` supplies safe picker metadata.
**Commit (previous baseline):** ccde05d — Initialize Francois template and artifact host research ticket.

### What I did

- Added Glazed serve, import, and models-list commands with tagged settings and structured model output.
- Added SQLite artifacts, immutable source versions, document operations, leases, assets, and comment records.
- Added loopback-only HTTP listeners: the library and each artifact have distinct ports, with Host and Origin validation.
- Implemented runtime injection before authored scripts, shared SSE transport, browser-owned tools, cancellation, cache/JSON helpers, capability namespaces, and a comments shell.
- Added a TypeScript Node worker using Pi ModelRuntime 0.85.1, its settings/model resolver, and complete provider continuation messages. No coding-agent bash/read/write/edit tools are loaded.
- Added React/Redux/RTK Query library and import UI with reference-derived CSS tokens.
- Added deterministic backend and tests for the call/tool boundary, storage conflicts/durability, rooted file ranges, and browser contract helpers.

### Why

The first milestone must exercise an actual page callback rather than stop at a text proxy. One event stream per view and a private NDJSON worker pipe keep callback ownership and provider authentication explicit. Separate artifact ports preserve localStorage isolation without changing imported application code.

### What worked

- Go tests passed, including an SSE/tool-result integration test.
- TypeScript check and browser bundles passed. Direct `node --import tsx tests/contracts.test.ts` reported three passing contract tests.
- Lint passed with zero issues after addressing unchecked Close calls, an unused type, and a predeclared name.
- The actual POLIS page rendered its scene, reported sample tools available, and executed the fake model's eval exactly once. `PolisIDE.presets.list()` then contained `Francois test`; the model result was “The browser tool completed successfully.”
- Real Pi model discovery succeeded using existing config/auth, without a paid inference request. The enabled scope selected openai-codex/gpt-6-astra for all tiers in the no-override configuration. This is visible model selection, not an assumption that tier labels imply distinct Claude models.

### What didn't work

- TypeScript initially rejected agent thinking level `off` as a model-stream reasoning level. Recovery maps off to undefined and marks worker initialization explicitly.
- A new view response referred to the removed variable `a`; Go reported `pkg/host/routes.go:129:56: undefined: a`. Recovery returns the actual loaded version and binds publishing to that baseline.
- A path test incorrectly treated `../bad` as an invalid JSON-document path. Document path segments allow dots and do not access the filesystem; the test was corrected, while project files continue to reject traversal.
- The first command-output test captured Cobra's writer, but Glazed serialized to stdout, leaving an empty JSON buffer. The test now captures stdout explicitly.
- Socket tests failed under the sandbox with `listen tcp6 [::1]:0: socket: operation not permitted`; the same authorized integration tests passed with loopback access.
- Initial lint found unchecked Close calls and an unused config type. Those findings were fixed together, then lint passed.
- The template baseline contained trailing whitespace in three workflow files. It was recorded by diff-check and removed in this implementation milestone.

### What I learned

Pi's stream type does not use the coding agent's `off` string. The model catalog exposes reasoning and image capabilities but does not expose a general tool-support boolean, so provider/tool behavior still needs real integration verification.

### What was tricky to build

The SSE handler must register a view before its call arrives, and tool results must resolve only that call's waiters. Page versions are supplied from the injected bootstrap, not replaced with the newest database version when creating a view. Tool children must be cancelled before waiting for their goroutines on terminal provider events.

### What warrants a second pair of eyes

Review cancellation/partial-text semantics, subscription optimism, same-person/different-tab room stamps, and initial versus reconnect view cleanup. Later phases still need browser acceptance evidence; presence of methods is not proof that every supplied contract detail is complete.

### What should be done in the future

Run real-provider streaming/tool smoke tests, add persistent capability fixtures and multi-tab tests, finish distribution/CI help, and update the support matrix to tested behavior. Image sampling, SVG upload, artifact files-form publish, custom comment anchors, and Claude-session/MCP integration remain explicitly unavailable.

### Code review instructions

Start with `pkg/host/server.go`, `pkg/host/routes.go`, `pkg/piworker/client.go`, `worker/src/main.ts`, and `ui/src/runtime.ts`. Review store transactions in `pkg/store`, then inspect the embedded host UI and browser helper tests. Run Go tests with writable cache and loopback access, plus pnpm check/build/test and lint.

### Technical details

Browser verification used a tmux server with backend fake, library at 127.0.0.1:8080, and POLIS at its persisted loopback port. All test data is under /tmp. Paid requests were not used for this milestone. Source snapshots remain immutable.

### Generation checkpoint correction

After the initial tests/lint passed, logcopter generation added the host package logger. The subsequent build reported `pkg/host/logcopter.go:7:5: log already declared through import of package log`. The milestone commit raced that final build result. I removed the redundant zerolog/log import and used the generated package logger, reran the affected build/tests, and amended the unpushed milestone. This failure shows why generation must precede final code validation.

## Step 5: Complete the tested local capability surface and package it

The Pi-backed browser test streamed a real response and executed one actual POLIS eval that changed the tower height from 6 to 7. The remaining local capability checks then verified shared document updates, optimistic snapshots, durable assets/comments, rooted files, room presence, and subscriber cancellation. The TypeScript source was formatted for review, and a local package target now includes the worker and its dependency manifest.

### Prompt Context

See Step 3 for the implementation request and Step 4 for the model-selection question.
**Commit (preceding milestone):** 0085786 — Implement local artifact host and Pi browser tool runtime.

### What I did

- Verified real Pi text streaming with openai-codex/gpt-6-luna: four text events and final text `Francois connected`.
- Verified a named-model tool round: the model invoked eval exactly once, patched `height: 6` to `height: 7`, and returned a final confirmation with modelApplied set.
- Added pending-write snapshots and rollback, query ordering/identity checks, bounded call queueing (two active/eight waiting), partial text for cancelled subscribers, and throttled text callbacks.
- Added image preprocessing: declared JPEG/PNG/WebP/GIF inputs, four-image/eight-MiB budget, roughly 1.2 MP resizing, JPEG re-encoding, source-byte cache hashes, and provider image messages.
- Added Pi selector thinking suffixes and safe supported-thinking-level metadata. Explicit effort takes precedence over the selector; `off` is mapped to absent provider reasoning.
- Kept lobby and named-room presence independent; removed null fields via actual null patches rather than silently leaving old state behind.
- Fixed project downloads to stream the whole file after confirmation instead of saving only one 25-MiB range read.
- Added a capability playground fixture and manifest.
- Added private worker protocol tests for tool-result exchange, cancellation, and invalid frames.
- Added Node/pnpm CI, frontend generation, local packaging, license comments in bundles, and a current README/support matrix.

### Why

The broad local API needs actual browser/state evidence, not just method names. The richer model-selection request fits Pi's existing resolver rather than requiring a second provider catalog or credential configuration system. Host packaging must include its Node worker instead of pretending the Go executable alone performs inference.

### What worked

- TypeScript check and four contract tests passed, including rejected optimistic-write rollback.
- Go tests and race-enabled tests passed across host/store/worker/commands. Worker pipe tests do not require live credentials.
- Lint passed with zero issues. Generation and build succeeded; make package-local produced the expected binary/worker/manifest/examples archive.
- Browser document data was frozen and nested updates preserved both fields. Pending snapshots were observed.
- Two same-origin browser documents received shared writes and represented two peers for one local identity. Root events had sameTab true; the second document had sameTab false and saw updated presence.
- Cancelling one shared sample subscriber preserved `first ` as partial text while its sibling completed `first second third`. JSON parsing and cached replay returned `{ok:true}`. Same-block abort rejected without a model call.
- Upload/list/fetch returned the stored text asset; data and comments survived a server restart. Rooted file reads returned size and content revision metadata.
- The comments shell opened a composer from the artifact, posted a comment through its UI, and displayed the persisted thread.
- A generated 1800×1200 PNG was sent as JPEG with dimensions 1342×894 (1,199,748 pixels).
- Named-room presence contained `{table:2}` while lobby presence contained `{lobby:1,view:'board'}`.
- Saved and visually inspected `reference/02-francois-library.png`; the white panels, fine borders, muted canvas and blue controls match the reference's visual language.

### What didn't work

- The first optimistic-query test expected reordered rows from a mock that did not declare orderBy. The runtime now enforces default ID ordering, so the fixture was corrected to order by rank; the test then passed.
- A broad source replacement for tool-schema validation also matched `c.names`, producing `c.if` and a Go syntax error. A focused patch restored the call registry loop while keeping validation in validateSample; build/tests passed afterward.
- The initial screenshot destination `/tmp/francois-library.png` was rejected by the browser tool's allowed roots. The screenshot was saved under the ticket instead.
- The sandbox pnpm launcher itself uses Node 20 and prints an engine warning even when its child PATH selects Node 24. Direct version verification returned v24.18.0; builds/checks/tests succeeded. The runtime/CI requirement remains Node 24.

### What I learned

Pi model selectors can express richer thinking levels without expanding the standard effort enum. A final-round empty string must remain distinguishable from missing finalText so sample.json never parses earlier tool-round prose. Range reads and downloads are different operations; sharing their implementation without a full-file path truncates exports.

### What was tricky to build

A cancelled subscriber must not cancel siblings sharing the same provider request. Optimistic snapshots need local overlays and a definitive refresh after success or rollback. Room presence requires separate server state per named room and null deletion patches. The browser owns binary transformations and callbacks; Go validates the wire budget and routes complete tool results.

### What warrants a second pair of eyes

Review origin/Host checks, lifecycle cleanup, and the renderer's terminal callback paths. The host is a single-owner localhost service; real organization identity, multi-account roles, original Claude shell/session semantics, and runtime definitions omitted from the downloaded guides are not claimed. SVG upload/custom anchors/files-form publish/MCP remain explicit follow-ups.

### What should be done in the future

Resolve the precise files-form publish and permissions type definitions before enabling them. Add SVG sanitization, custom comment anchors, and genuine conversation/MCP handoff only with concrete contracts. Production release infrastructure remains separate from this local package.

### Code review instructions

Review ui/src/runtime.ts and its capability modules, worker/src/main.ts, pkg/host/files.go and rooms.go, and pkg/piworker/client_test.go. Run pnpm check/test, go generate, go test -race, make lint, and make package-local. Compare the screenshot with Factory Twin tokens in pkg/web/static/style.css.

### Technical details

All manually created test databases live under /tmp. The real model smoke used explicit model mappings and the existing Pi auth configuration; credential values were not emitted. The local archive contains no credentials or test database. Unsupported methods are documented rather than shimmed under deprecated names.

## Step 6: Final review, worker readiness, and preview handoff

The completed capability milestone is committed as 263c006. Final review found that a dead worker could still advertise its startup model list. The client now returns an empty model list after protocol/process failure, and new sampling calls return sampling_disabled rather than entering a broken pipe. The existing protocol-failure test covers this state.

### Prompt Context

See Step 3.
**Commit (code):** 263c006 — Complete local capabilities and validate Pi hosting workflows.

### What I did

- Reviewed the final diff and confirmed source snapshots, credentials, and runtime data were not accidentally staged.
- Added worker readiness reporting and a protocol-failure assertion.
- Prepared persistent local imports for POLIS and the capability playground under ignored var/francois.
- Stopped the temporary test servers and prepared a Pi-backed tmux preview using explicit gpt-6-luna mappings.

### Why

The operator must see an unavailable worker as unavailable, while imported HTML and saved data remain accessible. Test data should remain separate from the user's persistent preview database.

### What worked

- Final race-enabled Go suite, TypeScript checks, four contract tests, lint, vet, generation, package-local, and docmgr doctor passed.
- A full-file download regression test streamed 25 MiB plus 3 bytes, proving the prior chunk truncation is fixed.
- The archive includes the Go executable, worker entrypoint, dependency lockfile/manifest, examples, and README.
- Git ignores the persistent database and package archive.

### What didn't work

The first focused patch assumed the Models method was multiline; apply_patch rejected its context without changing files. After reading the actual one-line method, the focused patch applied. Stopping a test server automatically removed its tmux session, so a subsequent kill-session reported that the session was absent; the listener was already stopped.

### What I learned

A successful startup handshake is a snapshot, not continuing worker readiness. Provider metadata must not remain advertised after a private protocol failure.

### What was tricky to build

Model readiness must be read without racing the event reader. The closed done channel is the lifecycle signal, so this needs no extra health polling or credentials checks.

### What warrants a second pair of eyes

The remaining extension tasks are explicitly open. The preview uses one local owner; provider costs are charged to that owner's Pi configuration.

### What should be done in the future

Use README for startup and the open ticket tasks for platform extensions. Production release infrastructure is not dispatched by this handoff.

### Code review instructions

Inspect Client.Models, the host's pre-call readiness check, and the invalid-frame worker test. The detailed design remains historical; README and ticket index describe the implemented state.

### Technical details

Preview runs in tmux session francois-preview on 127.0.0.1:8080 with Node 24 and explicit openai-codex/gpt-6-luna tier mappings. Artifact ports and SQLite identity are persisted in var/francois. No secret values or var data are committed.

## Step 7: Add POLIS LunaRoute model and thinking selection

POLIS now has separate model and thinking selectors for the requested LunaRoute models. The editable application is `examples/polis.html`; the original imported source snapshot is unchanged. The locally imported POLIS application was advanced from version 1 to version 2, preserving its existing port, identity, and original HTML version.

Validation exposed that LunaRoute's cached models and OAuth credential require provider registration normally supplied by its Pi extension. The worker now registers only the provider's cached catalog and key semantics, without loading extension commands, MCP, web tools, or login UI. Both requested models completed real browser requests with their exact selectors reported in `modelApplied`.

### Prompt Context
**User prompt (verbatim):** "add selection model for polis ide which is lunaroute / glm-5.3-flash low / medium / high thinking, and also deepseek4.1 low / high / xhigh (i think)"
**Assistant interpretation:** Add persistent browser controls for these two authenticated models and their specified thinking levels, and verify actual provider selection.
**Inferred user intent:** Choose the model used by POLIS's assistant directly rather than using Claude-branded tier names.

### What I did
- Added model/thinking state, persisted settings, valid-level normalization on model changes, and two selectors to `examples/polis.html`.
- Send the selected thinking level in the existing Pi model selector suffix; show applied model metadata on completed replies.
- Added `worker/src/lunaroute.ts` and invoked its provider registration before discovering available models.
- Used Pi's existing cached model catalog, provider base URL, and SDK credential resolution. OAuth access-key and no-op refresh behavior follow installed `@lunaroute/pi-extension` 0.7.2 `src/login.ts`.
- Published the editable HTML to the existing local POLIS artifact; restarted the preview in tmux with GLM quick/default and DeepSeek complex mappings.
- Documented setup and thinking mappings in README.

### Why
The original POLIS interface selected generic tiers and labeled them Haiku/Sonnet/Opus. Explicit provider/model suffixes already exist in Francois, so this change uses that path without changing the sampling contract. LunaRoute registration is necessary for the SDK to interpret its OAuth-stored gateway key and restore its dynamic catalog.

### What worked
- Inline HTML scripts passed Node syntax checks.
- `pnpm check`, `pnpm test` (four passing contracts), and `pnpm build` succeeded.
- Browser controls expose GLM low/medium/high and DeepSeek low/high/xhigh. Switching from GLM medium to DeepSeek normalizes to low, and reload restored DeepSeek xhigh.
- Live request `lunaroute/glm-5.3-flash:low` returned `connected` and the identical `modelApplied`.
- Live request `lunaroute/deepseek-4.1-flash:xhigh` returned `connected` and the identical `modelApplied`.
- Preview startup reports lunaroute GLM quick/default and DeepSeek complex; listener is 127.0.0.1:8080 and POLIS remains on port 33921.

### What didn't work
- Initial `go run ./cmd/francois models list --format json` under restricted access failed with `EROFS: read-only file system, mkdir '/home/manuel/.pi/agent/auth.json.lock'`. The authorized retry allowed SDK lock creation.
- The initial authenticated catalog contained no available lunaroute models. Direct ModelRuntime diagnostics showed only older static lunaroute entries and `available: []`. Reading cached catalog metadata established that the new models depend on the extension provider registration; the focused registration resolved this.
- An attempted new tmux preview exited because the existing preview was already running. Inspected the active listener, stopped it using `lsof-who -p 8080 -k`, and restarted the existing preview session.
- pnpm warns its launcher uses Node 20 although the runtime server explicitly uses Node 24. TypeScript/build/contracts succeeded; live calls used Node 24.

### What I learned
Pi model cache presence does not alone establish provider availability. Extension-defined OAuth providers need their provider registration even when authentication and catalog files already exist.

### What was tricky to build
GLM medium maps to provider high; DeepSeek xhigh also maps to high in the user's cached gateway model metadata. The controls retain the requested Pi levels while the existing SDK performs that mapping. Model changes must normalize a previously saved thinking level before the next sample.

### What warrants a second pair of eyes
Review provider registration against the installed extension's current key semantics if LunaRoute changes its authentication format. This integration intentionally restores the existing catalog offline; refresh and login remain Pi operations.

### What should be done in the future
Refresh the catalog in Pi when gateway models change. Additional model UI choices can use the same existing selector contract.

### Code review instructions
Start with `examples/polis.html` at MODELS, selectedThinking, sendToAgent, and Agent. Then review `worker/src/lunaroute.ts` and initialization in `worker/src/main.ts`. Compare the editable HTML against the unchanged ticket source to see the bounded application changes. Verify dropdown persistence at http://127.0.0.1:33921.

### Technical details
- Selectors: `lunaroute/glm-5.3-flash:<low|medium|high>` and `lunaroute/deepseek-4.1-flash:<low|high|xhigh>`.
- Persisted fields: `model` and `thinking` within the existing `polis-ide.v1` localStorage record.
- Provider catalog: `~/.pi/agent/models-store.json`; credential lookup remains private to Pi ModelRuntime.
- No credential values or local database contents were added to tracked files.
