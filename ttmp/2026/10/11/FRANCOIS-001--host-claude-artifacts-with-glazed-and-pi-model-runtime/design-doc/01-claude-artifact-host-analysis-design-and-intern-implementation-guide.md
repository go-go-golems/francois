---
Title: Claude artifact host analysis design and intern implementation guide
Ticket: FRANCOIS-001
Status: active
Topics:
    - backend
    - llm
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/main.go
      Note: Initialized Cobra and Glazed logging root
    - Path: repo://go.mod
      Note: Template module and dependency baseline
    - Path: repo://ttmp/2026/10/11/FRANCOIS-001--host-claude-artifacts-with-glazed-and-pi-model-runtime/sources/Artifact Claude API (sample) — Spec.md
      Note: Sampling and page tools contract
    - Path: repo://ttmp/2026/10/11/FRANCOIS-001--host-claude-artifacts-with-glazed-and-pi-model-runtime/sources/Artifact Files — Guide.md
      Note: Project reads and download contracts
    - Path: repo://ttmp/2026/10/11/FRANCOIS-001--host-claude-artifacts-with-glazed-and-pi-model-runtime/sources/Artifact People & Collaboration — Guide.md
      Note: Identity rooms and comments contracts
    - Path: repo://ttmp/2026/10/11/FRANCOIS-001--host-claude-artifacts-with-glazed-and-pi-model-runtime/sources/Artifact State Storage — Guide.md
      Note: Document publishing and asset contracts
    - Path: repo://ttmp/2026/10/11/FRANCOIS-001--host-claude-artifacts-with-glazed-and-pi-model-runtime/sources/Polis IDE.html
      Note: Browser model tool flow and actual app
ExternalSources: []
Summary: Evidence-backed local Go host design, browser tool bridge, Pi runtime integration, and phased intern guide.
LastUpdated: 2026-10-11T18:38:50.211181239-04:00
WhatFor: Implement the Francois artifact host with clear contracts and acceptance gates.
WhenToUse: Before implementing or reviewing the hosting service.
---

# Francois: Claude artifact hosting with Pi

## 1. What you are building

Francois is a local Go service that turns a saved HTML artifact into a working application. An artifact is an HTML page containing its own application logic, styles, and sometimes third-party script references. In Claude's viewer, a runtime supplies `window.claude.use(name)`. A downloaded page lacks that runtime. It may render perfectly but lose the buttons that ask a model, save shared state, or inspect files.

The service supplies that browser API and implements its operations locally. Go serves the page and owns application data. A small TypeScript worker uses Pi's installed model runtime to call providers using the owner's existing `~/.pi/agent` configuration. The browser keeps ownership of its JavaScript functions and DOM. Those boundaries explain almost every design choice below.

For example, open the supplied **Polis IDE**, type “make the towers shorter,” and press Send. The page sends instructions, conversation history, scene state, and one tool definition named `eval`. The model chooses a tool call. Francois delivers that call to the same browser tab. The tab executes its existing `eval` function, changes the city, and returns the result. The model then explains the change. Go must never execute the generated JavaScript as an operating-system command.

The first useful milestone is this complete POLIS flow. Subsequent milestones supply the other capabilities described in the four downloaded guides. The implementation should remain one Go process, one Node worker, one SQLite file, and a small browser library. A job queue service, a provider credential database, distributed rooms, and a generalized plugin framework would obscure the actual work.

This document is a design and implementation handoff. The repository has been bootstrapped; the host described here has **not** been implemented. Proposed paths and wire formats are marked as such. The downloaded guides are the supplied contract evidence, not independently verified Anthropic documentation. They describe contract 0.2.75 and must not be treated as proof that every current Claude deployment behaves identically.

### Read this guide in this order

1. Read sections 1–4 to understand the existing page and the system boundaries.
2. Read sections 5–9 before implementing the POLIS milestone.
3. Implement phases 1–3 and run the browser acceptance scenarios.
4. Read sections 10–13 as you add persistence and the remaining capabilities.
5. Use sections 14–17 for review, debugging, and exact source navigation.

## 2. Evidence and current repository state

The repository was initially an empty Git worktree with a docmgr configuration and templates. There was no Go module, application code, or remote. Initialization copied tracked files from the local `go-go-golems/go-template` checkout at commit `5bf14bfd4a7b13124dbe9f2c3078c29a33627a11`, replacing `XXX` with `francois`. The resulting module is `github.com/go-go-golems/francois`. There is one top-level `go.mod`.

The template provides GitHub Actions, Makefile targets, lint settings, lefthook, a logcopter generator and package logger, licensing, and GoReleaser configuration. `cmd/francois/main.go` now creates a Cobra root with Glazed logging, help, cancellation context, and version support. It does not start a server. The copied release configuration still describes the template's external publishing setup; no release credentials, Vault resources, remote repository, or deployment have been provisioned.

The newest four Markdown downloads by modification time were the following. Exact copies are under this ticket's `sources/`, with sizes and SHA-256 hashes in `sources/manifest.json`. Line references below refer to these copies, which are byte-identical to the originals when captured.

| Evidence label | File in `sources/` | Main contract |
| --- | --- | --- |
| S | `Artifact Claude API (sample) — Spec.md` | Sampling, streaming, tools, errors, JSON, MCP |
| D | `Artifact State Storage — Guide.md` | Page publishing, document database, assets |
| P | `Artifact People & Collaboration — Guide.md` | User identity, rooms, presence, comments |
| F | `Artifact Files — Guide.md` | Project file reads and browser downloads |
| H | `Polis IDE.html` | Actual target application and browser tool |

The guides are dated October 10, 2026. The HTML filename is **Polis IDE.html**, despite the uppercase name in the request. Its dependencies include Google Fonts, React 18.3.1, ReactDOM, and htm from external CDNs. Serving the file locally does not make those resources available offline.

### What POLIS already does

The HTML is roughly 3,600 lines and contains several independent parts:

- H:270–1915 defines `PolisCore`, the scene language, geometry, state, execution, and agent evaluation.
- H:1915 exports the core API to `window.PolisCore`.
- H:2588 exports model instructions and API documentation in `POLIS_DOCS`.
- H:2596–2963 implements and exports `PolisRender`.
- H:2970 onward implements the React IDE, editor, presets, console, and assistant chat.
- H:2976–2989 stores preferences, presets, and editor code in `localStorage` under the fixed key `polis-ide.v1`.
- H:3135–3143 acquires `sample`, calls `sample.limits()`, and disables the assistant if tools are unavailable.
- H:3189–3225 implements the assistant request and the `eval` browser callback.
- H:3571 initializes rendering and sampling from a React effect.
- H:3590 onward optionally uses `claude.hot` for development hot replacement.

`sample.limits().tools` is a hard readiness requirement. Supplying a text-only endpoint leaves the assistant in its “no tools” state. Supplying a function that always returns canned text does not satisfy the application either: the callback must operate on the actual `IDE` object in the user's tab.

The existing local storage is sufficient to demonstrate saved presets on one browser. POLIS currently does not call `db`, `assets`, `files`, `room`, `comments`, or `user`. It does not need a collaboration backend before its assistant can work. Its tier labels say Haiku, Sonnet, and Opus; the local host can map those tiers to other providers, so those labels are descriptive UI text rather than an enforceable model identity.

### What Pi already provides

The installed package is `@earendil-works/pi-coding-agent` version **0.85.1**, located at:

```text
/home/manuel/.nvm/versions/node/v24.18.0/lib/node_modules/
  @earendil-works/pi-coding-agent/
```

Its `docs/sdk.md:370–499` documents model discovery, authentication, and custom file locations. Its `dist/core/model-runtime.d.ts:1–102` defines `ModelRuntime.create`, model discovery, authentication, `streamSimple`, and `completeSimple`. Its `dist/core/settings-manager.d.ts` defines settings loading and getters. This installed API uses `ModelRuntime`; examples based on older Pi `AuthStorage` / `ModelRegistry` combinations are not the correct primary implementation reference here.

The inspected configuration contains `auth.json`, `models.json`, `models-store.json`, and `settings.json`. Stored authentication includes both OAuth and API keys. `models.json` has custom provider definitions and legacy top-level entries; the worker must let Pi decide which entries are supported instead of independently interpreting all of them. The configured default and the enabled-model list may differ. Never assume the default is selectable just because its strings appear in the file.

Only structure, provider names, credential types, and model-selection fields were inspected. Credential values were not printed or copied into this ticket. The implementation must preserve that property.

## 3. Essential concepts and ownership

A **capability** is a named JavaScript namespace or function obtained through `claude.use`. A manifest declares which capabilities a page needs. A **view** is one loaded browser document. A view owns callback functions, subscriptions, its page-version baseline, and an opaque view identifier. Reloading creates a new view; opening two tabs creates two views.

An **artifact** is a stable record containing an ID, manifest, current page version, source files, and durable data. A **version** is an immutable published page revision. The same artifact may have many views. A **sample call** is one memoryless model conversation owned by one view; the page supplies the entire history each time. It may require several provider rounds because tools cause the model to ask again with tool results.

A **person** is a durable local identity; a **peer** is a temporary room participant for one view. Two tabs owned by the same person have one person ID and two peer IDs. Pi provider authentication identifies the host owner to the model provider. It is not a browser login mechanism and must not be used to derive artifact roles.

```text
Browser tab: HTML application + embedded runtime
  |  requests, stream reads, tool results, cancellation
  v
Go service: views, manifests, authorization, calls, storage
  |  NDJSON requests/events on private stdin/stdout
  v
Node worker: Pi ModelRuntime + settings + model round loop
  |  provider protocol and OAuth/API-key handling
  v
Model provider

Go service ---- SQLite: artifacts, versions, docs, comments
Go service ---- data directory: immutable blobs/source objects
Browser tab --- its own callbacks, DOM, canvas, local storage
```

“NDJSON” means one JSON object per newline. A complete line is one frame; logs go to stderr so they cannot be mistaken for protocol frames. SQLite supplies transactions and durable local records. It does not change the browser API's last-writer-wins semantics into a transaction API.

## 4. Scope and architectural decisions

### Decision: use Pi's model runtime in a dedicated worker

- **Context:** The requested configuration includes OAuth credentials and custom models, not only static API keys.
- **Options considered:** Reimplement provider/auth handling in Go; invoke the Pi coding CLI per call; use Pi's TypeScript model runtime.
- **Decision:** Use a small worker importing `ModelRuntime` and the Pi settings utilities.
- **Rationale:** Pi already owns provider composition, token refresh, model catalogs, and configured headers. The coding CLI introduces coding tools and session semantics that a memoryless artifact call does not require.
- **Consequences:** Distribution requires Go plus Node and the pinned worker package. Embedding browser assets produces one Go web binary, not a self-contained model runtime binary.
- **Status:** proposed implementation choice, grounded in the installed SDK.

The worker uses `ModelRuntime.streamSimple` directly. It does not create a coding session, discover skills, load extensions, or expose Pi's bash/read/write/edit tools. Only functions explicitly passed by the page become model tools. A provider requiring an extension-registered runtime is outside the first milestone; report it unavailable instead of silently loading unrelated extensions.

### Decision: preserve the requested page contract

The page-facing API is expressly requested by the user. Implement those capability methods as the service's public contract; do not create deprecated aliases such as `self`, old `window.claude.sample`, or older Pi API facades. Browser methods and Go wire methods need not have identical names. The wire protocol is an internal design owned by Francois.

### Decision: localhost ownership first

Start on `127.0.0.1`, with one persistent owner identity and multiple browser tabs. The owner pays all model costs through Pi. Unknown capabilities resolve to `null`. Missing browser capabilities do not block the page's ordinary rendering.

The design covers multi-view data and room behavior locally. Internet multi-user hosting requires a separate browser identity mechanism, role grants, and per-artifact access enforcement. Do not present a server bound to all interfaces as a production sharing service. Do not advertise multi-user roles while accepting a caller-supplied person ID as authentication.

### Decision: one shared view event stream

Use one server-sent event stream per view plus ordinary HTTP POST commands. SSE is a long-lived HTTP response carrying named events. The browser reads server events; POSTs carry actions in the opposite direction. This meets streaming, tools, subscriptions, room events, and publish notifications without a second transport.

Prefer this to a WebSocket initially because requests, cancellation, and binary upload/download are already HTTP operations. A connected view is required for sample calls with tools. Reject requests before spending model usage if the return path is unavailable.

## 5. Command line and package layout

Implement `serve` as a Glazed **BareCommand**: it runs a process rather than producing a table. Implement `models list` as a **GlazeCommand**: it emits structured rows and gets `--format`, `--output-fields`, and `--max-output-rows`. `import` can also emit one structured row describing the registered artifact. Do not manually reproduce Cobra parsing for these commands.

Proposed command examples:

```sh
go run ./cmd/francois import --file './Polis IDE.html' --id polis

go run ./cmd/francois serve \
  --listen 127.0.0.1:8080 \
  --data-dir ./var/francois \
  --pi-agent-dir /home/manuel/.pi/agent \
  --worker ./worker/dist/main.js \
  --model-quick provider/model \
  --model-default provider/model \
  --model-complex provider/model \
  --log-level debug

go run ./cmd/francois models list --format json
```

These are **future commands**, not executable features of the scaffold today. Defaults should be fields in command descriptions. Expand `~` explicitly when resolving paths; Node APIs do not universally expand it. Go can use `os.UserHomeDir` to supply a default without calling `Getenv`.

| Flag | Meaning |
| --- | --- |
| `listen` | Local HTTP address; default `127.0.0.1:8080` |
| `data-dir` | SQLite and local source/blob storage |
| `pi-agent-dir` | Pi configuration directory; default expanded `~/.pi/agent` |
| `worker` | Path to built worker entrypoint; an executable path, never a shell command |
| `model-quick/default/complex` | Explicit provider/model mapping for each tier |
| `project-root` | Optional read-only root for the `files` capability |
| `log-level` | Glazed/zerolog logging setting |

Keep credentials out of flags and browser-visible model rows. Parse model selectors in the worker with Pi's resolver, not by splitting an arbitrary string on every slash. Models can have provider-specific identifiers.

Proposed file layout:

```text
cmd/francois/main.go                 existing root
cmd/francois/cmds/serve.go           BareCommand and settings decode
cmd/francois/cmds/import.go          source registration
cmd/francois/cmds/models/list.go     structured model inventory
pkg/host/server.go                  http.Server lifecycle, ServeMux
pkg/host/views.go                   view creation and event connection
pkg/host/routes.go                  concrete HTTP handlers
pkg/runtime/contracts.go           JSON wire structs and errors
pkg/runtime/calls.go                call ownership and terminal state
pkg/piworker/client.go              NDJSON subprocess client
pkg/artifacts/service.go            manifest/version/import operations
pkg/store/sqlite.go                 persistent records and migrations
pkg/store/query.go                  JSON document query semantics
pkg/rooms/hub.go                    temporary peers and event fan-out
pkg/files/service.go                bounded read-only filesystem access
pkg/web/embed.go                    import _ "embed"; static assets
pkg/web/static/runtime.ts           page-facing capability API
pkg/web/static/transport.ts         POST + SSE, event dispatch
pkg/web/static/sample.ts            functions/callbacks/cache/JSON
pkg/web/static/db.ts                refs, snapshots, subscriptions
pkg/web/static/rooms.ts             peer state and listeners
worker/package.json                pinned Pi dependency and scripts
worker/pnpm-lock.yaml               reproducible dependency resolution
worker/src/main.ts                 stdio dispatcher
worker/src/runtime.ts              Pi config and selected models
worker/src/sample.ts                provider round loop
examples/polis/manifest.json        sample capability declaration
```

Add packages only when a phase introduces them. Do not fill this tree with empty abstractions. Interfaces belong at actual seams: a provider backend enables deterministic sampling tests; a clock enables lease expiration tests. Concrete types are otherwise sufficient.

Use `github.com/pkg/errors` for Go error wrapping, context arguments for I/O, zerolog package loggers, and `errgroup` for worker/server goroutines. Include compile-time interface assertions such as `var _ cmds.BareCommand = (*ServeCommand)(nil)`.

## 6. Importing and serving HTML

Import reads a source file once, stores its original bytes, records a manifest, and creates version 1. It does not serve the Downloads directory wholesale. POLIS's default example manifest is `{"sample": {}}`. A sidecar manifest is authoritative because capability use in JavaScript cannot be reliably inferred from text search.

Serve application documents through a handler that injects the runtime before any artifact script executes. Store source independently from the injected response so republishing never persists host-generated bootstrap scripts. Use an HTML tokenizer to locate the head insertion point; support the actual POLIS head and full-document HTML fixtures. Do not run blind regular-expression replacement over JavaScript containing HTML strings.

A proposed injected prefix is:

```html
<script type="application/json" id="francois-view">
{"artifactId":"polis","version":1,"viewId":"v_example"}
</script>
<script src="/static/francois/runtime.js"></script>
```

JSON must escape `<` as `\u003c` so malicious strings cannot close the script element. The runtime script must execute synchronously before authored scripts. It immediately installs a frozen `window.claude` object with `use`; `use` remains asynchronous and returns stable frozen capability handles. POLIS calls it later from React, but simpler artifacts may call during initial execution.

### URLs and static files

Use only `net/http` and `*http.ServeMux`. A small host navigation UI, if added, uses Bootstrap and React/TypeScript with Redux and RTK Query for its own server state. Do not rewrite the existing POLIS UI to use Bootstrap or Redux: its supplied code already owns its scene state.

```text
GET  /                                  small host index
GET  /artifacts/{id}/                    injected application document
GET  /static/francois/runtime.js         embedded host runtime
GET  /static/artifacts/{id}/{path...}     imported static dependencies
POST /api/artifacts/{id}/views           create view/session binding
GET  /api/views/{view}/events             SSE
POST /api/views/{view}/calls              start sample
POST /api/views/{view}/calls/{call}/cancel
POST /api/views/{view}/calls/{call}/tools/{tool}/result
POST /api/views/{view}/rpc                non-stream capability operations
POST /api/artifacts/{id}/assets          bounded binary upload
GET  /_blob/{blob}                       required asset API URL
```

`/artifacts/{id}/` is a functional HTML document endpoint. Static dependencies belong under `/static/`, not under arbitrary functional paths. `/_blob/` is the explicit asset contract from D:223, not a general static-file mount; its handler resolves artifact ownership and content headers before returning bytes. There must be no broad directory server exposing the data directory or Pi configuration.

Relative references inside imported pages need attention. POLIS references absolute CDN URLs, so injection is enough for its dependencies. For multi-file artifacts, import an explicit bundle, rewrite resource references using parsed attributes, and map public resource URLs into `/static/artifacts/{id}/...`. Browser `fetch('data/doc.json')` is not an HTML attribute; document the bundle URL base and require authored data fetches to use that base. Do not pretend an import can rewrite arbitrary JavaScript safely.

### Browser storage isolation

Different URL paths on one origin share `localStorage`. Claude's original per-artifact origin isolation therefore does not arise from path routing. For milestone 1, host one artifact per origin and use POLIS's existing key unchanged. Once multiple artifacts are admitted, give each artifact a dedicated local origin, for example a distinct loopback port served by the same Go process, with its API on that origin too. Keep a control index on the primary port. Do not patch the global Storage API or silently allow two POLIS imports to overwrite each other's presets.

This is an explicit delivery gate for multi-artifact support, rather than a complicated origin scheme in the first POLIS milestone. The host must bind each listener to a known artifact, validate Host headers, and use origin-scoped sessions. Port selection and persistence belong in artifact registration. A page version retains the same origin.

## 7. Browser API and sampling contract

The runtime exposes only `window.claude.use(name)` in its basic API. Optional POLIS `claude.hot` is omitted; the page feature-detects it. `use('sample')` resolves to a callable function with frozen `.json` and `.limits` properties. Disabled, undeclared, and unsupported capabilities resolve to `null`. In the first milestone, `sample` is the only enabled capability.

| Page operation | Browser responsibility | Go/worker responsibility |
| --- | --- | --- |
| `sample(input, options)` | Validate/copy input; keep callbacks/signal; normalize request | Own call; resolve model; provider rounds |
| `sample.json(input, options)` | Distinct verb; parse completed text | Return text and termination metadata |
| `sample.limits()` | Resolve local manifest/model support snapshot | Supply honest limits at view initialization |
| `onText({text, delta})` | Invoke callback with accumulated visible text | Send text deltas across rounds |
| `options.tools` | Retain `execute` functions in call registry | Forward names/descriptions/schema; return invocations |
| `options.signal` | Abort locally and POST cancellation | Cancel context and provider AbortController |

Never JSON-serialize a function or AbortSignal. The runtime makes a serializable request containing input, scalar options, and tool descriptors. The callbacks stay in a per-call Map in the originating browser view. Validate unknown tool names and duplicate tool IDs before invoking any function.

### Inputs and limits

Accept a non-empty string or a user/assistant turn array starting and ending with user. Merge consecutive same-role turns. Count **UTF-8 bytes**, not JavaScript string length, against 262,144 bytes. Copy input at invocation, before the next microtask; clone serializable descriptors so a subsequent page mutation cannot alter the request. All `sample` failures are Promise rejections with plain `{code, message, text?}` objects. Reference-building methods in `db` have a different synchronous-error contract.

The first release should accept `modelTier`, `model`, `effort`, `cache`, `signal`, `onText`, and `tools`. Image support is a separate tested phase. When not implemented or when the selected model lacks image input, omit `limits().images` and reject supplied images with `images_unavailable`. Never advertise support from the manifest alone.

Use `{maxPromptBytes: 262144, tools: {maxCount: 16}}` as a proposed local tools limit, subject to selected model support. The contract does not specify an exact tools count; this value is a Francois policy. A model without tool support cannot satisfy POLIS readiness. Reject it before opening a provider request.

### Result and termination

Return the documented shape:

```typescript
interface SampleResult {
  text: string;
  truncated: boolean;
  modelTierApplied: 'quick' | 'default' | 'complex';
  modelApplied?: string;
}
```

`modelApplied` is included only when the explicit requested model was used. Keep the actual provider/model in host logs and model inventory; do not invent new meanings for the page field. `truncated` is true for a length limit or a completed deadline cutoff with useful text. An interrupted transport or aborted call rejects, carrying partial text. An empty final response rejects `empty_completion`.

Pi's stop reasons include `stop`, `length`, `toolUse`, `deferred`, `aborted`, and `error`. Complete ordinary stop and length paths; service toolUse through the browser bridge. The first implementation does not implement deferred jobs: fail with a useful `upstream_error` and diagnostic rather than treating a deferred handle as final text. Do not claim a provider refusal unless its returned evidence actually identifies one.

### JSON and cache

For `.json`, parse the complete text, then one Markdown fence, then the outer candidate object/array span. Return the parsed JSON value without schema validation. A truncated or unparsable reply rejects `invalid_json` with raw text. Parse only the final assistant round when tools are used; preceding explanatory text is not the JSON body.

Cache only successful calls without tools. The key includes normalized input, requested tier/model/effort, verb, and eventual image bytes. Default retention is five minutes, capped at 24 hours. `cache: false` disables it. `cache: {refresh: true}` bypasses replay and replaces the entry after success. A storing call and a reading call both constrain freshness through the shorter `gcTime`.

Identical concurrent calls without tools can share a provider request, but each subscriber retains its own abort state and partial text. Cancelling one subscriber must not abort a remaining subscriber. Tool-bearing calls are never shared or cached. If `cache` is explicitly true/object with tools, reject as `invalid_request`; omission is allowed and treated as no cache. Implement this after the basic tool flow; it is not required to make POLIS's current call work because POLIS omits cache when supplying its tool.

## 8. HTTP event protocol and call state

The following is a proposed internal wire format. Browser API fidelity is mandatory; this particular protocol is an implementation choice.

After the bootstrap creates a view, the runtime opens SSE and waits for `ready`. It then posts actions with request IDs. The server binds the view to an HttpOnly local session and artifact; IDs alone are not authorization. The call-start handler returns `202` with the accepted call ID. Events can arrive before the POST resolves, so register the pending call before sending the request.

```json
{"id":"c_12","input":[{"role":"user","content":"Make it shorter"}],
 "options":{"modelTier":"quick"},
 "tools":[{"name":"eval","description":"Run scene edits",
           "inputSchema":{"type":"object","properties":{"code":{"type":"string"}},"required":["code"]}}]}
```

SSE uses `event: call` and JSON `data` frames:

```json
{"callId":"c_12","type":"text","text":"I will","delta":"I will"}
{"callId":"c_12","type":"tool","toolCallId":"tc_1","name":"eval","input":{"code":"ide.lines()"}}
{"callId":"c_12","type":"result","result":{"text":"Done.","truncated":false,"modelTierApplied":"quick"}}
{"callId":"c_12","type":"error","error":{"code":"cancelled","message":"Stopped","text":"I will"}}
```

The last two frames are alternatives; a call settles exactly once. Tool result POST:

```json
{"value":"{\"ok\":true,\"lines\":\"...\"}","isError":false}
```

Maintain this state machine:

```text
created -> queued -> running -> waiting_for_tools -> running
   |          |         |             |                |
   +----------+---------+-------------+----------------+
                        |
           done OR failed OR cancelled (terminal)
```

A sample deadline bounds the entire operation and per-round provider time; tools receive a 150-second child signal as described in S:147. A proposed maximum is five rounds, with the last round sent without tools to solicit a final answer. If that round still cannot produce a final response, return a bounded failure with any partial text. These are configurable host policies, not measured provider guarantees.

Allow two active calls and a small queue, for example eight waiting calls. Reject further work with `rate_limited`. No worker should block the shared stdio reader while waiting for a browser callback; requests and tool-result frames need concurrent dispatch.

On view disconnect, cancel its live calls. Reconnect re-establishes subscriptions with fresh authoritative snapshots and peers; it does not replay an uncertain browser `eval`, republish, or other write. An SSE stream has a bounded event buffer. A slow consumer gets disconnected and follows the same recovery rule. A timed-out request may already have performed browser effects; communicate interruption rather than claiming rollback.

### Browser pseudocode

```typescript
function sample(input, options = {}) {
  // Capture now; report validation errors through the returned Promise.
  let call, validationError;
  try { call = captureAndValidate(input, options); }
  catch (error) { validationError = asSampleError(error); }
  return Promise.resolve().then(async () => {
    if (validationError) throw validationError;
    if (call.signal?.aborted) throw cancelled();
    await transport.ready();
    if (call.signal?.aborted) throw cancelled();
    register(call);                    // before POST, events may race response
    attachAbort(call);
    try {
      await transport.post('calls', call.serializableRequest);
      return await call.completion;
    } finally {
      abortAllToolChildren(call);
      unregister(call);
    }
  });
}

async function handleTool(call, event) {
  const definition = call.tools.get(event.name);
  const signal = childSignal(call.signal, 150_000);
  try {
    const value = await definition.execute(event.input, { signal });
    assertStringOrPlainJSON(value, 32_768);
    await postToolResult(call, event.toolCallId, value, false);
  } catch (error) {
    await postToolResult(call, event.toolCallId,
                         'Error: ' + safeMessage(error), true);
  }
}
```

The captured call owns copies of arguments, while validation errors become asynchronous rejections. Abort in the same synchronous block prevents any network send. Tool child signals must abort both when the call settles and when their deadline expires; cleanup must remove deadline timers and abort listeners.

Promise cancellation cannot forcibly interrupt a synchronous JavaScript loop. POLIS's `C.evalAgent` executes page code in the main document (H:1657 onward); a blocking generated program can freeze its tab. Do not promise that an AbortSignal can terminate it. That is a browser application execution issue, distinct from terminating a model request. A future POLIS-specific worker evaluator would require changing its DOM/IDE dependencies and is outside the host milestone.

## 9. Pi integration and the provider round loop

Pin `@earendil-works/pi-coding-agent@0.85.1` in a TypeScript worker package and install with pnpm. Type-check against that installed API. The worker uses Pi's own transitive `pi-ai` message types and provider runtime; do not independently pin mismatched package versions. Direct type imports need compatible explicit package dependencies if pnpm's strict resolution requires them.

### Configuration responsibilities

| File | Owner/use | Handling |
| --- | --- | --- |
| `auth.json` | Pi credential storage: OAuth/API keys | SDK only; never emit values |
| `models.json` | Provider/model configuration and custom headers | SDK loader; no second parser |
| `models-store.json` | Cached remote catalogs | Pi model store; no secret-free guarantee assumed for arbitrary content |
| `settings.json` | Preferred provider/model/thinking and model scope | Pi settings getters; no mutation |
| `extensions/`, `skills/`, packages | Coding-agent behavior | Not loaded by this model worker |

Proposed initialization:

```typescript
import { ModelRuntime, SettingsManager,
         resolveCliModel, resolveModelScopeWithDiagnostics }
  from '@earendil-works/pi-coding-agent';

const runtime = await ModelRuntime.create({
  authPath: join(agentDir, 'auth.json'),
  modelsPath: join(agentDir, 'models.json'),
  modelsStorePath: join(agentDir, 'models-store.json'),
  allowModelNetwork: false,
  signal: startupAbort.signal,
});
const settings = SettingsManager.create(isolatedWorkerCwd, agentDir);
const available = await runtime.getAvailable();
const scope = resolveConfiguredScope(settings.getEnabledModels(), available);
const tiers = chooseTierModels(explicitFlags, settings, scope);
emitReady(publicModelSummaries(tiers));
```

`resolveConfiguredScope` and `chooseTierModels` are proposed helpers, not Pi exports. Use `resolveModelScopeWithDiagnostics` for actual wildcard/model matching. SettingsManager may also load project settings from its `cwd`; use an isolated host-owned worker directory without an artifact `.pi/settings.json` to avoid letting HTML imports select configuration. Confirm the trusted-project options in the pinned SDK before implementation.

By default ModelRuntime restores cached catalogs without network refresh. OAuth refresh during inference can still require network access and may update Pi's credential file. Do not copy OAuth credentials into a separate host store. Propagate SDK refresh errors as service errors, never as credential values.

Pi's documented credential priority includes environment fallback after stored credentials, and custom model keys can involve environment or command resolution. This is existing SDK behavior, not something the Go host should silently duplicate with `Getenv`. Document it in `serve --help` and startup diagnostics without values. Review the SDK's environment override options when a file-only policy is requested; do not assert that an empty override map prevents every internal environment access without testing it. The Go service's own settings come from Glazed fields, not environment scraping.

### Tier selection

Use explicit tier flags first. Otherwise use the authenticated configured default if it meets the required model capabilities and selected scope. If only one authenticated tool-capable model is eligible, all tiers may map to it; document that tiers then change requested reasoning effort, not model price or latency. If no eligible model exists, report a startup/model-selection error. Do not silently pick an arbitrary expensive model.

The inspected default and enabled-model list differ, which makes this rule material. A requested named model is resolved through Pi. If it is unavailable, the supplied sampling contract permits fallback to the requested tier; report the applied tier and omit `modelApplied`. Do not use fictional sample IDs such as the guide's example model names as defaults.

`effort` low/medium/high maps to the model runtime's supported reasoning option; quick defaults to reasoning off where supported. Pi settings provide the baseline thinking level. Providers can ignore unsupported reasoning options. Validate the selected model's supported features instead of assuming every provider behaves like Anthropic. No provider identity or API key belongs in page JavaScript.

### Worker protocol

The Go parent starts `node worker/dist/main.js` with explicit argv, cwd, and config paths. No `sh -c`. Node stdout carries frames; stderr carries logs. Go reads stdout continuously, routes frames by call ID, and serializes writes so concurrent goroutines cannot interleave JSON.

```json
{"type":"init","protocol":1,"agentDir":"/home/manuel/.pi/agent"}
{"type":"ready","protocol":1,"models":[{"tier":"quick","provider":"example","id":"tool-model","tools":true}]}
{"type":"sample","callId":"c_12","input":"...","options":{},"tools":[]}
{"type":"text","callId":"c_12","delta":"Hello"}
{"type":"tool","callId":"c_12","toolCallId":"tc_1","name":"eval","input":{"code":"ide.lines()"}}
{"type":"tool_result","callId":"c_12","toolCallId":"tc_1","value":"...","isError":false}
{"type":"cancel","callId":"c_12"}
{"type":"result","callId":"c_12","result":{"text":"Hello","truncated":false,"modelTierApplied":"quick"}}
```

Use a length-bounded buffered reader or raise Scanner's token limit explicitly; Go's default Scanner limit is smaller than the allowed prompt. Cap frames based on the protocol envelope, for example 2 MiB for text-only requests. Images should travel through explicit bounded binary staging rather than endlessly raising line limits.

On worker exit, fail every in-flight call once, release waiters, and mark model readiness false. Do not automatically retry provider operations: a prior request may have spent usage or performed browser effects. A worker may restart for subsequent manually initiated requests after initialization succeeds.

### Round-loop pseudocode

```typescript
async function runSample(request, signal) {
  const context = { messages: toPiTurns(request.input),
                    tools: toPiTools(request.tools) };
  let visible = '';
  for (let round = 0; round < MAX_ROUNDS; round++) {
    if (round === MAX_ROUNDS - 1) context.tools = [];
    const stream = runtime.streamSimple(modelFor(request), context,
                                         optionsFor(request, signal));
    for await (const event of stream) {
      if (event.type === 'text_delta') {
        visible += event.delta;
        emitText(request.callId, visible, event.delta);
      }
      // Do not forward thinking text or partial tool JSON as page text.
    }
    const message = await stream.result();
    assertNotErrorOrAborted(message);
    context.messages.push(message);       // preserve full provider message
    const calls = completedToolCalls(message);
    if (!calls.length) return finalResult(message, visible, request);
    const results = await Promise.all(calls.map(async call => {
      emitTool(request.callId, call);
      return await waitForBrowserResult(request.callId, call.id, signal);
    }));
    context.messages.push(...toPiToolResultMessages(calls, results));
    appendRoundSeparatorIfTextWillFollow();
  }
  throw boundedToolLoopError(visible);
}
```

Preserve full assistant messages, including non-visible provider reasoning/signature blocks, for the next round. Rebuilding them as `{role:'assistant', content: visibleText}` loses tool IDs and provider-specific continuation data. A Pi tool-result message includes `role:'toolResult'`, `toolCallId`, `toolName`, text/image content, `isError`, and timestamp (installed `pi-ai/dist/types.d.ts:330`). The page sees visible text from all rounds separated by blank lines, while `.json` parses only final-round text.

Use model-completed tool arguments, not the fragments from `toolcall_delta`. Tool execution failures become error tool results and let the model continue. Invalid tool names or protocol ownership are host errors, not invitations to invoke arbitrary functions. Request budgets include added tool results and assistant messages before every subsequent provider round.

### POLIS sequence diagram

```text
POLIS page       Go host       Pi worker          Provider
    |               |              |                 |
    | sample + eval descriptor     |                 |
    |-------------->| sample frame |                 |
    |               |------------->| stream request  |
    |               |              |---------------->|
    |               |              |<----------------|
    |               |<-------------| tool eval(args) |
    |<--------------| SSE tool     |                 |
    | C.evalAgent(code, IDE)       |                 |
    | tool result   |              |                 |
    |-------------->| result frame |                 |
    |               |------------->| next round      |
    |               |              |---------------->|
    |               |              |<----------------|
    |<--------------|<-------------| text + result   |
    | update chat; city already changed              |
```

The browser-side callback makes this integration fundamentally different from a chat proxy. Test that path first.

## 10. Durable state, publishing, and assets

Add SQLite after the first sampling milestone. The service owns one database file under `data-dir`. Use SQL transactions internally for atomic updates, but expose the document API described in D:142–201 rather than a SQL API. Page callers never receive filesystem paths or database handles.

A minimal schema is enough:

```sql
CREATE TABLE artifacts (
  id TEXT PRIMARY KEY,
  manifest_json TEXT NOT NULL,
  current_version INTEGER NOT NULL,
  origin_port INTEGER UNIQUE
);
CREATE TABLE versions (
  artifact_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  source_hash TEXT NOT NULL,
  author_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (artifact_id, version)
);
CREATE TABLE documents (
  artifact_id TEXT NOT NULL,
  path TEXT NOT NULL,
  body_json TEXT NOT NULL,
  version INTEGER NOT NULL,
  PRIMARY KEY (artifact_id, path)
);
CREATE TABLE leases (
  artifact_id TEXT NOT NULL,
  path TEXT NOT NULL,
  holder TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  data_json TEXT,
  PRIMARY KEY (artifact_id, path)
);
CREATE TABLE assets (
  id TEXT PRIMARY KEY,
  artifact_id TEXT NOT NULL,
  object_hash TEXT NOT NULL,
  content_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  created_at TEXT NOT NULL
);
```

Add source-file version maps when implementing multi-file publishing, and comment/thread tables in the comments phase. Use bound parameters for SQL. Store original sources and blobs under hashes in a private object directory; asset IDs and source hashes are separate concepts. Durable metadata references immutable bytes, avoiding a half-overwritten source file when a view reads during publication.

### Document API

`db.doc(path)` and `db.collection(path)` create pure references without I/O. Reject invalid path shapes synchronously with TypeError. Documents have even segment counts, collections odd counts. Preserve the guide's path character/size/depth limits (D:97). Add IDs client-side for `.doc()` with no argument and `.add(data)`; request IDs let retries recognize the same operation without creating a second document.

Bodies are JSON objects of at most 256 KiB and 32 levels. `set` replaces; `update` recursively merges objects while arrays/scalars replace; update of a missing document rejects `invalid_argument`; delete is idempotent and leaves nested documents alone. These details are observable behavior, not storage implementation choices.

Queries use top-level fields, up to ten filters, one order, and a limit of 1–1,000. Fetch immediate children of the collection, apply authorization, then evaluate filters and sorting. Do not include descendants just because their path shares a prefix. Preserve missing-field ordering and deterministic ID tiebreaks. Specify cross-type comparison behavior in contract fixtures before accepting mixed-type queries; the guide does not define a complete ordering for unlike JSON types. Start with a scan of small collections instead of inventing indexing machinery.

A write increments an internal version counter and commits before broadcasting. Subscribers receive fresh query evaluations after commit. `onSnapshot` must deliver an initial snapshot asynchronously, return unsubscribe synchronously, and maintain stable frozen snapshots/data for unchanged documents. Query change records identify added/modified/removed documents and indexes. Implement an identity cache keyed by artifact/path/version in the browser runtime.

For local writes, emit an optimistic snapshot with `hasPendingWrites:true`, then an authoritative one when the request commits. On rejection, restore the committed state and invoke the error path. A simple per-document serial write queue matches the API's one-write-at-a-time rule. Keep the server's subscription registration and initial snapshot ordering atomic with respect to its write dispatcher, or a write between reading and subscribing can disappear.

### Lease acquisition

`acquire` is a cooperative lease, not a data write lock. In one SQLite transaction, examine the current lease; acquire if missing, expired, or renewable by the same holder. Use a server clock and clamp TTL to 1 second–10 minutes, default 30 seconds. Return `{acquired:false}` for a busy lease. Callers must still coordinate read/set themselves; normal writes do not become protected merely because a lease exists.

```text
BEGIN IMMEDIATE
  lease = read(path)
  if lease is unexpired and lease.holder != holder:
      return acquired:false
  replace lease(holder, now + boundedTTL, data)
COMMIT
return acquired:true
```

### Authorization and local identity

A local owner meets all ordinary roles. Still write rule evaluation as a small explicit function so tests can cover future view/interact/admin identities. Default shared reads require view and writes interact. Deeper rules override inherited levels. A `{self}` rule and the `data/users/<uid>` privacy rule must compare authenticated identity, not a UID passed in request JSON.

Read-denied documents appear missing and are excluded from queries; write-denied operations reject `invalid_argument`. Owner status does not expose another person's private subtree. Enforce visibility before counting query results and returning cursor state. Treat `user.can()` as UI advice; every server operation checks authorization independently.

### Publishing the page

`artifact.publish(html)` is a compare-and-set against the version loaded by the view. It is not “overwrite whatever is current.” Validate a complete document, write immutable source bytes, then atomically insert a new version and advance the artifact row only if the expected version matches. Broadcast reload to every view after commit.

```text
expected = view.loadedVersion
object = storeValidatedHTML(originalSubmittedBytes)
transaction:
  UPDATE artifacts SET current_version = expected + 1
    WHERE id = artifact AND current_version = expected
  if affected != 1: fail conflict
  INSERT versions(artifact, expected + 1, object.hash, viewer, now)
after commit: publish reload event to all views
```

A conflict does not trigger an automatic republish loop. The losing view reloads the winner. Retain the original source separately from injected HTML; DOM serialization would include temporary state and host code.

The supplied guide's files-form `publish` description mentions text/null entries and per-file `ifMatch` without a full typed argument structure (D:63). Do **not** invent an exact browser signature and claim contract fidelity. Implement whole-HTML publishing first; resolve the files-form type definitions before enabling it. Once resolved, compare hashes only for touched paths so concurrent edits to different files can both succeed, reject fixed type-owned paths, and preserve untouched files. Until that gate is satisfied, return a method-level `capability_removed` for files-form publishing rather than silently applying last-writer-wins.

### Assets

`assets.upload(blob,{type?})` accepts only the guide's MIME types and size limits; text must be UTF-8, SVG must be sanitized, and upload is writer-only. Store bytes, register a durable asset ID, and return `{id,url,sizeBytes,contentType}`. Use `/_blob/<opaque-id>` as the stable public shape. `list` returns metadata and usage; delete returns `{deleted}` and is idempotent when absent.

An asset upload and a later document write are not a transaction. If the document write fails, the uploaded asset may be orphaned; `list()` makes it discoverable for housekeeping. Do not silently delete it under a guessed ownership rule. Use safe content types and `nosniff`; serve code/data assets as downloads or deliberate resources rather than executing arbitrary uploaded HTML. SVG sanitization needs a tested allowlist, not removal of the first `<script>` substring.

An early asset phase may explicitly omit SVG until its sanitizer is implemented. Unsupported input rejects `unsupported_type`; supported raster/data assets must still survive restart and respect artifact ownership.

## 11. Project files and downloads

The `files` capability maps a declared logical project to one configured read-only filesystem root. It does not expose `~/.pi/agent`, the host data directory, or arbitrary absolute paths. The first milestone does not need it. The files phase must provide real range/revision behavior before advertising the capability.

| Method | Local implementation |
| --- | --- |
| `list({path,recursive,cursor})` | Sorted relative entries, at most 500/page; cursor bound to listing parameters |
| `read(path,{offset,length})` | Bounded binary HTTP response plus size/revision metadata; browser wraps Uint8Array |
| `download(path)` | Host/runtime name-and-size confirmation followed by browser download |
| `asSampleTools()` | Browser functions wrapping list/read; readable text clipped near 12 KB |

Use Go's rooted filesystem facilities available in the selected toolchain, or a carefully reviewed path-open implementation that rejects traversal and symlink escape. A string-prefix comparison against an absolute root is insufficient: `/project-other` and symlinks can pass careless checks. Treat `..`, absolute paths, platform separators, directory reads, negative ranges, and stale cursors as test cases.

Read no more than 25 MiB per call. The response revision must describe the bytes actually read, not just an earlier stat. Re-stat after reading, detect mid-read changes, and use a stable revision scheme tested under overwrite and rename. A content hash is strong but costs a full read; a metadata-derived revision is cheaper but needs an explicit guarantee and change checks. For this local service, use immutable captured read objects where practical and return `changed` for inconsistent reads. The page can compare revisions between chunks.

`downloads.save` is browser-owned. Accept strings, ArrayBuffers, views, and Blobs, confirm the final filename and size, and create a browser download after approval. Use `showSaveFilePicker` where supported and a confirmed object-URL link otherwise. A custom confirmation dialog supplies the capability's decision step; a browser download alone is not the required prompt. Revoke object URLs when done.

Preserve typed-array copy behavior and ArrayBuffer detachment semantics through transfer where supported; failing to detach is a known behavioral difference that must be caught in tests rather than ignored. Save extension allowlists and sanitized names come from F:110–148. `saved` means handed to the browser, not proven written to disk. Platform-request export tokens and `delivered` require an actual host export workflow; ordinary saves should omit `request`, and unsupported tokens reject `request_unknown`.

For a host confirmation UI, use a small Bootstrap modal owned by the runtime, keep one pending save prompt, and return `declined` without retries. Do not embed a file upload or download permission system into every artifact's application code.

## 12. User, rooms, and comments

These capabilities require honest local semantics. They cannot reconstruct Claude's organization directory or original conversation from an exported HTML file.

### User identity

For localhost, persist one opaque owner ID under the host's data directory and return a local display profile. `user.id`, `me`, `profiles`, `name`, `email`, `avatarUrl`, `isOwner`, `canEdit`, and `can` follow P:25–106. Reads never reject: unavailable values use documented blanks/nulls/false, and `can` can return null when the host has no advice. Scopes control fields. A local email is unavailable unless deliberately configured.

Profile search can return the configured local identities, capped at eight. It is not a search of Claude users. No names or emails are obtained from OAuth tokens. Profiles are resolved at render time; durable document relationships store IDs. If real sharing is added later, provision authenticated local people and role grants rather than asking a browser to announce it is an editor.

### Room and presence

Use an in-memory hub keyed by artifact ID and room name. Room records disappear after disconnect and are rebuilt on reconnect. Durable presenter state belongs in `db`; temporary cursor position belongs in room presence.

| Operation | Key behavior |
| --- | --- |
| `emit` / `on` | Broadcast to all current subscribers including sender; no history replay |
| `presence` | Merge patch; top-level null deletes; max merged object 4 KiB |
| `peers` | Synchronous frozen snapshot with stable identity until change |
| `onPeers` | Batched callback with peers/joined/left/updated |
| `join` / `leave` | Independent named channels; lobby remains joined; max 16 |
| `connected` / `onConnection` | Current transport state and change callbacks |

Stamp `peer`, `by`, `isMe`, and timestamps on the server. `by` is null when user identity is not declared. Do not trust a sender ID from a presence patch. Topics are declared with role levels in the manifest. Presence is data, not proof of role.

Coalesce presence at about 30 updates/second and render changes no more than once per animation frame. The hub drops events while disconnected. Restore the latest local presence and joined rooms on reconnect, then deliver current peers. Do not replay old reactions. Remove a view from all rooms when its SSE closes or heartbeat expires.

`sendToClaudeSession` is a Claude-shell operation, not a model completion. Return an unavailable status until Francois has a real conversation composer and an explicit Pi-session handoff design. Never silently turn “stage a draft” into a paid sampling request. The method may reject `claude_unavailable` while the rest of room remains operational.

### Comments and a host shell

The supplied comments capability is write-only because Claude's shell displays threads. Francois must own that display surface if it advertises comments. Add a small host shell with a Bootstrap comment panel and an artifact iframe, plus a defined postMessage bridge for composer/anchor operations. Check source window and origin for every shell message. For the initial POLIS milestone, there is no iframe or comments shell: serve the document directly.

Implement composer-only first: `anchorFor` and `openComposer`. Serialize a DOM anchor or quoted range; never attempt to send an Element or Range object over JSON. A stable element ID is preferable to an unstable generated CSS path. The shell displays threads, while the page remains free of a thread-list API.

Full comments add create/reply/resolve/delete with 4 KiB text limits and server role checks. Store thread/reply IDs, author IDs, anchors, timestamps, and resolved state in SQLite. Writes happen only on gestures and text is rendered as plain text. If full comments are enabled, `sendToClaude` and its availability result must reflect an actual host conversation integration; otherwise report `no_session`/`off` and `claude_unavailable` rather than fabricating a successful reply.

Custom anchors and interception callbacks have their own lifecycle (P:227–239). They are later extension work after the shell exists. Their omission should be explicit in the support matrix. Returning null for entirely absent comments is better than returning a namespace whose every method pretends to succeed.

### MCP and permissions

S:218–272 describes connectors that live in Claude's viewer. The local host has no such connector registry. In the initial release, `use('mcp')` resolves null. Existing Pi `mcp.json` is not automatically a Claude connector manifest, and this design does not load it. If local MCP support is later required, define a separate allowlisted tool registry, connection lifecycle, and consent model rather than treating connector names as executable commands.

Implement local `permissions.state`/`request` only after checking the original typed contract: the supplied sample guide names them but does not specify all signatures or result shapes. A host-owned “enable local model usage” control can gate sampling internally without claiming a fabricated permissions API. Availability discovery is free; the first paid sampling action should make the owner-cost behavior clear. No Claude account authorization prompt can be recreated from Pi credentials.

## 13. Explicit support matrix and delivery gates

A host must tell an author what works. Capability acquisition is binary at the namespace level, but some documented optional methods/features require additional discovery or honest error/status paths. The table below is the planned progression, not a claim of current implementation.

| Capability | First POLIS milestone | Later gate |
| --- | --- | --- |
| sample text + streaming | Required | All normalization/error cases |
| sample page tools | Required | Concurrent tools and bounded rounds |
| sample limits | Required, honest tools availability | Images by selected model |
| sample JSON + caching | After first tool milestone | Parsed final round, coalesced abort semantics |
| user | Null initially | Persistent local owner/scopes/defaults |
| db | Null initially | Queries, rules, snapshots, leases, restart |
| artifact whole HTML | Null initially | Version CAS and reload broadcast |
| artifact files form | Unsupported initially | Obtain precise typed signature, per-file CAS |
| assets | Null initially | Binary persistence, ownership, MIME limits, sanitizer |
| files | Null initially | Explicit root, traversal protection, revisions/ranges |
| downloads | Null initially | Confirmation, allowlists, binary semantics |
| room | Null initially | Multiple views, no replay, presence/join cleanup |
| comments | Null initially | Host shell; composer, store, moderation |
| Claude-session handoff | Unavailable | Explicit composer/session integration |
| MCP | Null | Separately scoped connector implementation |
| permissions API | Unavailable | Obtain typed contract before exposing |
| claude.hot | Absent | Development-only feature if requested |
| deprecated aliases | Absent | No requirement to add them |

The complete ticket includes implementing supported local capabilities, not merely drafting their names. Deferred platform-specific features must remain individually visible tasks. Do not close the implementation ticket because POLIS can talk to a model; do not delay that useful milestone while building unrelated comments features.

## 14. Intern implementation plan

### Phase 1: preserve a functioning bootstrap

Read the current root, module, Makefile, workflows, and generated logger. Confirm `go run ./cmd/francois --help`, `--version`, tests, build, generation, and lint. Add a Glazed BareCommand for serve with a tagged settings struct and `DecodeSectionInto(schema.DefaultSlug, ...)`. Add a simple handler using `http.ServeMux` and `http.Server`.

Server lifecycle pseudocode:

```text
settings = decode Glazed values
ctx = command context with OS signal cancellation
open persistent directories
start worker and await ready with startup deadline
construct services and register routes
listen explicitly; log actual address
errgroup:
  HTTP Serve(listener)
  worker reader / exit watcher
  on ctx.Done: HTTP Shutdown(bounded context); stop worker
wait; normalize http.ErrServerClosed; return meaningful error
```

Do not leave detached servers running from a command invocation. Use tmux for manual interaction. Acceptance: root CLI works, server stops on Ctrl-C, a fake worker error reaches the caller, and no secrets appear in logs.

### Phase 2: serve POLIS with an embedded runtime

Copy or import the ticket's HTML fixture as source bytes. Declare only sample. Build `runtime.ts` into an IIFE browser bundle with pnpm so no import statement executes after the page scripts. Embed built assets with Go's `_ "embed"` import. Inject bootstrap using parsed HTML and prove the script executes before authored scripts.

Implement `use`, `sample.limits`, a deterministic fake sampling backend, the shared SSE view connection, and error settlement. Acceptance: POLIS loads its city and reports sampling ready with tools; a fake `eval` invocation changes editor code and produces a recorded tool result. The fixture's original bytes/hash remain unchanged by serving.

### Phase 3: real Pi sampling and tools

Build the pinned worker. Load configuration using ModelRuntime and Pi settings. Add model-list output with safe metadata. Implement provider text streaming, complete tool messages, browser result waiters, cancellation, and terminal errors. Make limits depend on the selected model.

Acceptance: a user asks for a small city edit; a provider requests `eval`; POLIS shows a successful call, the scene changes, and the assistant receives the result before producing its final answer. Stop cancels network work and clears host waiters. A closed view does not leave a pending provider round. The test must inspect behavior, not merely HTTP 200 status.

Use a paid real-provider smoke only with an explicit small prompt and the configured owner's model; deterministic protocol tests cover the rest. No paid inference was performed during this research handoff.

### Phase 4: finish sample contract and origin isolation

Add tolerant JSON parsing, cache keys/TTL/refresh, subscriber-level cancellation, quotas, effort/tier mappings, image declarations and transformations if included, and multi-artifact origins. For images, implement MIME checks, image count/byte limits, metadata stripping and resizing according to the supplied contract; report exact local limits through `.limits`.

Acceptance: malformed arguments reject asynchronously; input is copied at invocation; same-block abort makes no provider request; tool cache policy is correct; invalid JSON is not cached; separate POLIS imports do not share presets. No feature claims success by discarding an unsupported option.

### Phase 5: documents and page versions

Add schema migrations, identity, rule evaluation, refs/queries/snapshots, subscription fan-out, cooperative leases, and whole-page publication. Write a small tracker/checklist fixture modeled on D's examples. Resolve the files-form API definition before adding it.

Acceptance: two tabs observe writes; data survives restart; update merges nested fields correctly; hidden reads look missing; query changes maintain correct indexes; competing publishes have one winner and all views reload to it; original HTML remains free of runtime injection.

### Phase 6: files, downloads, and assets

Add the bounded project reader, explicit root declaration, revision semantics, sample file tools, confirmed downloads, and asset storage. Acceptance: ranges and bytes match fixtures; forbidden paths never open; cancelled saves stay cancelled; uploaded assets work after reload; orphaned uploads are listed; MIME and SVG checks match enabled support.

### Phase 7: room and comments shell

Add local multi-tab presence and room events, then a shell for comments. Add user profile methods and scoped defaults. Keep Claude conversation handoff unavailable until implemented. Acceptance: two tabs have distinct peers for the same owner; reactions echo once and do not replay; reconnect restores presence; unsubscribes and close remove listeners; comments render as plain text in the shell and moderation is enforced server-side.

### Phase 8: handoff and operational readiness

Update help and the support matrix to actual behavior, add a clear local-only runbook, include the worker in packaging, and review release configuration independently from feature development. The inherited template uses external publishing destinations and credentials; replace that setup only when release work is explicitly undertaken, using the project's standard release workflow. Do not ship a release that silently omits its required Node worker.

Record actual command output, versions, fixture hashes, and known unsupported platform features in the diary. Complete implementation tasks only with their acceptance evidence.

## 15. Test strategy and review checklist

The tests should exercise boundaries that can fail in realistic ways. A test that copies a helper's implementation logic into its assertion offers little confidence. Use a fake worker/provider to generate deterministic text, tool calls, pauses, errors, and crashes. Use browser tests because functions, signals, frozen objects, DOM anchors, and ArrayBuffer transfers cannot be validated by Go tests alone.

| Boundary | Test scenario | Required observation |
| --- | --- | --- |
| Glazed settings | Defaults and explicit listen/model flags | Typed settings and help-visible flags |
| Runtime load order | Artifact calls use from first script | Object present synchronously; use resolves asynchronously |
| Tool execution | `eval` calls `ide.lines` then `ide.patch` | Both execute in the initiating tab, not Go/Node |
| Tool failures | Page callback throws | Model receives error result and may continue |
| Interleaving | Two active calls, same tool name | Correct call and tool IDs; no callback cross-talk |
| Cancellation | Before POST, mid-text, during tools | No early spend; partial text; no leaked waiter |
| SSE failure | Close tab during tool round | Provider cancelled; no automatic tool replay |
| Worker failure | Invalid frame / process exit | Calls fail once; later readiness reflects failure |
| JSON | Fenced JSON, invalid text, truncated reply | Correct parsing/errors; only final tool round parsed |
| Cache | Same input, changed effort, refresh, one subscriber abort | Correct keys and shared ownership |
| Storage | Shared write/restart/missing update | Correct durability and API errors |
| Query | Missing fields, moving ordered limit window | Deterministic snapshots and change indexes |
| Rules | Hidden documents/private subtree | Missing-style reads, rejected writes |
| Publish | Two writers with same baseline | One CAS winner and reload signal |
| Files | `..`, absolute path, symlink, change between ranges | No escape; proper range and revision behavior |
| Download | Decline, invalid extension, buffer transfer | No silent save; documented binary behavior |
| Rooms | Two tabs, reconnect, named rooms | Distinct peers, no replay, correct cleanup |
| Artifact origins | Two imports of POLIS | No shared fixed-key localStorage |

Model runtime tests should use temporary non-secret configuration and a fake provider. Configuration selection tests must cover default outside enabled scope, OAuth unavailable, custom model unsupported, and no tool-capable model. Do not snapshot actual auth files or include provider headers in logs.

Manual server runbook for the future implementation:

```sh
lsof-who -p 8080 -k
tmux new-session -d -s francois-dev \
  'go run ./cmd/francois serve --listen 127.0.0.1:8080 --log-level debug'
tmux capture-pane -p -t francois-dev -S -150
# Interact with the browser, then inspect output.
lsof-who -p 8080 -k
tmux kill-session -t francois-dev
```

Adjust the command to include the actual worker/config paths. When multiple artifact ports are enabled, record them and use `lsof-who -p <port> -k` for each. Tests should leave no processes or open worker pipes.

### Everyday debugging

If POLIS says sampling is unavailable, inspect bootstrap order, `use('sample')`, and manifest registration. If it says tools are unavailable, inspect the selected model's features and the returned limits. If text arrives but the scene never changes, inspect call/tool IDs, event connection ownership, browser `eval` errors, and the tool-result POST. If the worker hangs after a successful browser edit, inspect its pending-result Map and whether the full assistant/tool messages were appended to the next provider request.

If a response fails only after a second model round, check context byte limits, provider continuation blocks, and unsupported tool arguments. If saved presets appear in another imported page, inspect origin assignment. If writes vanish on reload, verify the data directory and commit-before-broadcast ordering. Do not fix any of these by automatically retrying writes or discarding errors.

## 16. Practical limits and unresolved contract details

The simple local architecture deliberately separates actual implementation work from missing platform information. These are specific gates, not a reason to block POLIS hosting:

- The four downloaded guides are user-supplied descriptions. Precise `permissions` signatures and artifact files-form types need original runtime definitions before fidelity can be claimed.
- Claude's organization identity, viewer-specific subscription billing, connector credentials, and conversation shell do not exist locally. Francois uses the owner's Pi provider access and local identities.
- Pi's configured extensions may define additional providers; the model-only worker does not load them by default. Report missing providers and add narrowly scoped integration only when needed.
- Browsers cannot forcibly abort a synchronous page evaluator. Provider cancellation is supported; browser JavaScript termination is not promised.
- External CDN resources make the supplied POLIS file network-dependent. Vendoring those resources is separate, straightforward packaging work if offline operation is required.
- Multi-artifact origin isolation is required before advertising independent browser storage. A single-artifact localhost milestone can ship first.
- Provider refusals, reasoning support, deferred responses, image limits, and tool support vary. Translate actual SDK evidence; do not invent Anthropic-specific behavior for every provider.
- The inherited release scaffold is not proof of a configured production release. The design's worker distribution must be included when release work happens.

A reviewer should distinguish three completion claims: scaffold initialized, design delivered, and host implemented. This handoff completes the first two. All implementation phases remain open until tested.

## 17. API and file reference map

The source snapshots are the primary references for the supplied artifact behavior. Paths below are relative to the ticket unless an absolute path is shown.

| Topic | Primary reference |
| --- | --- |
| Capability loading and async behavior | `sources/Artifact Claude API (sample) — Spec.md:23–49` |
| Prompt normalization/JSON/limits | Same file, lines 51–108 |
| Sampling options/cache/tools | Same file, lines 110–175 |
| Results and error codes | Same file, lines 177–216 |
| MCP connector API | Same file, lines 218–272 |
| Browser storage/page publishing | `sources/Artifact State Storage — Guide.md:13–82` |
| Document paths/rules/privacy | Same file, lines 84–140 |
| Document/query/snapshot/lease APIs | Same file, lines 142–201 |
| Asset MIME, limits, methods | Same file, lines 203–253 |
| Claude-side seed/version operations | Same file, lines 255–278; separate from page API |
| User profile/role/identity behavior | `sources/Artifact People & Collaboration — Guide.md:25–106` |
| Room events/presence/lifecycle | Same file, lines 108–195 |
| Comment shell/anchors/session methods | Same file, lines 197–252 |
| File reads/ranges/errors | `sources/Artifact Files — Guide.md:17–99` |
| Download/save formats and errors | Same file, lines 101–148 |
| Actual POLIS model bridge | `sources/Polis IDE.html:3135–3228` |
| POLIS evaluator | `sources/Polis IDE.html:1657` |
| Current CLI root | Repository `cmd/francois/main.go` |
| Go/template generation | Repository `go.mod`, `Makefile`, `logcopter_generate.go` |
| Pi model runtime API | Installed package `dist/core/model-runtime.d.ts` |
| Pi model/auth usage examples | Installed package `docs/sdk.md:370–499` |
| Pi settings API | Installed package `dist/core/settings-manager.d.ts:163,199–217,310` |
| Provider message/event shapes | Installed package `node_modules/@earendil-works/pi-ai/dist/types.d.ts:330,389–465` |

Upstream entry points, consulted for provenance:

- [go-go-golems/go-template](https://github.com/go-go-golems/go-template)
- [Pi coding-agent source and documentation](https://github.com/earendil-works/pi/tree/main/packages/coding-agent)
- [Pi SDK documentation](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/sdk.md)
- [Glazed command interfaces](https://github.com/go-go-golems/glazed/blob/main/pkg/cmds/cmds.go)
- [Go net/http](https://pkg.go.dev/net/http)

The installed Pi 0.85.1 files own the precise API claims in this guide. Upstream main can change after this handoff. Before implementing, compare the pinned package's exported types and keep the lockfile aligned. The exact downloaded evidence is retained independently of those upstream changes.
