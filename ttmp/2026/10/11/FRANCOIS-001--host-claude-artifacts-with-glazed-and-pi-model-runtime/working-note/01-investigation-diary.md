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
