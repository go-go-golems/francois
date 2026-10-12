# Francois

Francois hosts saved HTML artifacts and supplies their Claude-style browser APIs
using your local Pi configuration. The library uses a compact light interface
inspired by Factory Twin: gray canvas, white panels, fine borders, blue accents.
Imported applications keep their own styling.

## Run locally

Requires Go (the module selects its toolchain), Node 24+, and pnpm 10.13.1.

```sh
pnpm install --frozen-lockfile
pnpm build

go run ./cmd/francois import \
  --file '/path/to/Polis IDE.html' --id polis --title 'Polis IDE'

go run ./cmd/francois serve --log-level info
```

Open `http://127.0.0.1:8080`. Each imported artifact has its own persistent
loopback port, keeping browser storage separate. You can also import HTML from
the library UI. Stop the process with Ctrl-C. Artifact data is in
`./var/francois/francois.db`; do not delete it to restart the service.

Update an existing artifact from an HTML file:

```sh
go run ./cmd/francois update --id polis --file examples/polis.html
```

This publishes a new immutable HTML version and preserves the ID, title, port,
capability manifest, documents, assets, comments, and browser storage origin.
Refresh open pages to load the new HTML. `import` still creates a new artifact
and requires a unique ID; `update` requires an existing ID.
The file is the source of the complete replacement HTML: keep application changes
in that file, such as the model controls and error messages in `examples/polis.html`.

The model worker loads `~/.pi/agent/auth.json`, `models.json`, `models-store.json`,
and Pi settings. Pi handles OAuth and provider configuration. Its SDK may use
credential environment fallback and configured key resolvers; Francois does
not copy credentials into its database or browser. Coding-agent extensions,
skills, shell tools, and existing Pi coding sessions are not loaded.

Model tiers follow the authenticated Pi default when it is in the enabled scope,
otherwise the first available model in that configured scope. Choose explicit
mappings when you want different models:

```sh
go run ./cmd/francois serve \
  --model-quick openai-codex/gpt-6-luna \
  --model-default openai-codex/gpt-6-sol \
  --model-complex openai-codex/gpt-6-astra

go run ./cmd/francois models list --format json
```

Model selectors must exist in your configuration and have authentication.
An artifact can request a model directly with
`sample(prompt, {model: 'provider/model', effort: 'high'})`.
Pi selectors can include thinking levels, such as `provider/model:xhigh`.
Explicit `effort` takes precedence. `/api/models` includes supported thinking levels.
Unavailable explicit models fall back to the tier; `modelApplied` identifies a
successful explicit selection and `modelUsed` reports the actual provider/model.
POLIS shows a fallback notice when the requested selector is not applied. The owner’s Pi account pays for all model calls.

## POLIS model selection

`examples/polis.html` is the editable POLIS application with model and thinking
selectors. The ticket's imported HTML remains an unchanged source snapshot.
Import this example with `--manifest examples/polis.manifest.json`.
The model selector offers `lunaroute/glm-5.3-flash` (low/medium/high) and
`lunaroute/deepseek-4.1-flash` (low/high/xhigh), persisting both choices in the
artifact's localStorage. It sends the selected thinking level as a Pi model suffix.
The configured gateway maps GLM medium to high and DeepSeek xhigh to high.
POLIS streams provider thinking into a collapsible, auto-scrolling panel above
its answer. The sample runtime exposes `onThinking({text, delta})` and a separate
`result.thinking` field; thinking stays out of answer text and chat history.
Providers that emit no thinking text simply show the normal answer.
Answers and thinking render GitHub-flavored Markdown, including tables and fenced
code blocks. The POLIS HTML loads pinned Marked and DOMPurify browser scripts
from jsDelivr, alongside its existing CDN dependencies; rendered HTML is sanitized.

The worker registers LunaRoute's provider using Pi's cached `models-store.json`
catalog and existing Pi credentials, including its OAuth-stored gateway key.
Log in and refresh the catalog through Pi first; Francois does not load the
LunaRoute extension's MCP, web tools, settings UI, or login workflow.

## Capabilities

Import accepts `--manifest capabilities.json`, or a manifest in the library UI.
The default is `{"sample":{}}`. Use
`examples/capabilities.manifest.json` with `examples/capabilities.html` to exercise
the local board, presence, assets, and comments. Enable rooted project-file reads
with `serve --project-root /path/to/project` and declare `files` in the manifest.

Implemented local surface:

- `sample`, streaming, browser tools, stop, JSON parsing, memory caching,
  named models, effort, declared images, and local limits. Two active calls and eight queued calls.
- `db`: JSON documents, merge updates, queries, frozen snapshots, subscriptions,
  and cooperative leases. Data is shared across tabs and survives restart.
- `artifact.publish(html)`: immutable page versions, compare-and-set, reload.
- `assets`: raster/data/font/video uploads, listing, deletion, and durable URLs.
- `files`: explicit read-only root, range reads, content revisions, listing,
  downloads, and sample tool wrappers.
- `downloads`: confirmed browser saves with filename/extension checks.
- `user`: one durable local owner and scoped local profile information.
- `room`: multi-tab peers, presence, named rooms, and ephemeral events.
- `comments`: composer and persisted threads in the optional `/shell` host UI.

This is a localhost owner service. It does not reproduce Claude organization
accounts or viewer-specific billing. Unsupported features return absence or
explicit errors: SVG asset upload, files-form publishing,
custom comment anchors, conversation/session handoff, MCP connectors, and the
underspecified permissions namespace. Cache entries are in browser memory and
clear on reload. Normal browser downloads remain subject to browser handling. Declare
`{"sample":{"images":true}}` to enable up to four JPEG/PNG/WebP/GIF images
(8 MiB input total). The runtime resizes to roughly 1.2 megapixels, strips
metadata by canvas re-encoding, and sends JPEG to an image-capable model.

## Development and verification

```sh
pnpm check
pnpm test
pnpm build
go generate ./...
go test ./...
go build ./...
make logcopter-check
make lint
```

`--backend fake` selects the deterministic offline backend. It never calls a
provider. Its POLIS test tool saves a preset called `Francois test`; prompts
`fixture:stream` and `fixture:json` exercise streaming and JSON behavior.

Run servers in tmux and stop listeners with `lsof-who -p <port> -k`.
The browser and worker sources live in `ui/src` and `worker/src`.
Generated browser assets are tracked in `pkg/web/static` so a plain Go build
can embed them; `pnpm build` refreshes them and builds `worker/dist/main.js`.
CI checks the browser assets against the lockfile and runs contract tests.

`make package-local` produces `dist/francois-local.tar.gz` containing the Go
binary, Node worker, examples, and dependency manifest. Unpack it, install
runtime dependencies with `pnpm install --prod --frozen-lockfile`, then run
`./francois serve` from that directory. Node is still required.
The inherited template release workflow is not configured or deployed.

Initialized from `go-go-golems/go-template` commit
`5bf14bfd4a7b13124dbe9f2c3078c29a33627a11`.
Detailed design, evidence, tasks, and implementation diary are in docmgr ticket
**FRANCOIS-001**, under `ttmp/2026/10/11/`.
