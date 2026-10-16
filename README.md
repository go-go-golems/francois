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

## Hosted subscriber catalog

`francois catalog` is a separate, multi-user service: subscribers sign in through
ZITADEL (OIDC code + PKCE), subscribe through Stripe-hosted Checkout, and run
release-pinned application instances. Each instance executes on its own origin
(`i-<id>.<app-domain>`), never on the trusted portal/admin origin, and receives
the same `window.claude` runtime with server-enforced, instance-scoped `db`,
`room`, `user`, `assets`, `comments` and quota-controlled `sample` calls served by
the operator's Pi configuration. The local `serve` command is unchanged.

Run the whole stack locally (Linux; uses host networking so the browser and the
server share `http://localhost:18081` as the issuer):

```sh
docker compose -f deploy/compose/compose.yaml up --build
docker compose -f deploy/compose/compose.yaml logs zitadel-setup   # test users
```

Open `http://localhost:8090` and sign in as `admin@francois.test`,
`alice@francois.test` or `bob@francois.test` (password `Password1!`). Optional
settings go in the ignored `deploy/compose/.env`:

```sh
STRIPE_API_KEY=sk_test_...        # sandbox key; enables the stripe profile
STRIPE_PRICE_ID=price_...         # recurring Price; list it in a plan (Admin → Settings → Plans)
FRANCOIS_BACKEND=pi               # default: fake deterministic model
PI_AGENT_DIR=/home/you/.pi/agent  # provider files are copied, read-only source
```

With Stripe: `docker compose -f deploy/compose/compose.yaml --profile stripe up -d`.
The forwarder's signing secret is handed to the server through the bootstrap
volume; only verified webhooks change access.

Operator commands (all take `--database-url` or `--database-url-file`):

```sh
francois catalog migrate
francois catalog serve --public-url https://francois.example --app-domain apps.example ...
francois catalog db publish --artifact-id polis --version 1.0.0 --html examples/polis.html --actor-user-id usr_...
francois catalog db users promote --issuer https://issuer --subject <sub>   # bootstrap admin
francois catalog db users grant-access --user-id usr_... --days 30 --reason pilot [--plan key]
francois catalog db settings --file settings.json                       # validated revision
```

One process can serve the same catalog under further domains. Each
`--site <portal origin>=<app domain>` adds a portal and its instance domain;
the app domain may equal the portal hostname (`https://slop.example=slop.example`
puts instances at `i-<id>.slop.example`, so one `*.slop.example` certificate
covers both). Sign-in, the Origin check, launch codes, framing and Stripe
return URLs follow the domain a request arrives on, and `www.<portal>`
redirects to the portal. Register `<portal>/auth/callback` and `<portal>/` on
the OIDC client for every site. Sessions are per domain.

Visitors see a server-rendered storefront: front page, catalog (`/apps`),
artifact pages with write-ups, images and videos, creator pages, a journal
with RSS (`/journal`) and a pricing page (`/pricing`). Artifacts can offer
short-lived demos to visitors without an account.

Creators and administrators work in the studio (`/studio`): artifact details,
page drafts with a Markdown preview, releases (HTML upload → check → draft →
isolated preview → publish/withdraw) with attachments (manuals, archives,
notes, downloadable under the release's rule and changeable after
publishing), demos, journal posts, creator profiles, a media library and
personal API tokens. Videos (MP4, WebM) work as covers, gallery items and
Markdown embeds (`![caption](/media/<id>)`), with a poster taken from one of
their frames. A creator edits the artifacts credited to their profile.
Personal API tokens are created on the account page (`/account`) with
scopes: `account` (the owner's account, billing links and instances),
`studio` (content the owner may edit), and for administrators `admin:read`
and `admin:write` (the admin API; these tokens expire within 90 days). A scope
never exceeds the owner's role, `GET /api/me` reports a token's identity and
scopes, and changes made through a token name it in the audit log.
`francois catalog releases push` uploads releases with a `studio` token, without
database access:

```sh
francois catalog releases push --server https://catalog.example --token-file token \
  --artifact-id polis --version 1.2.0 --html dist/polis.html \
  --attach docs/manual.pdf --publish
```

The admin UI (`/admin`) covers users (roles, disable, session revocation,
complimentary access for a plan, credit adjustments), instances (inspect,
suspend), billing events and reconciliation, model usage, versioned settings
(approved models, thinking levels, credit weights, plans with Stripe prices
and limits, demo limits, storefront wording) and the audit log. Plans decide
which artifacts an account may run (by rank), how many instances and members,
and how many model credits.

The production deployment (yolo k3s, `francois.yolo.scapegoat.dev` and
`slop.boutique`) is described in [doc/production.md](doc/production.md).

Integration tests need a PostgreSQL server; each test creates its own database:

```sh
TEST_DATABASE_URL='postgres://user:pass@127.0.0.1:5432/postgres?sslmode=disable' go test ./pkg/catalog/...
```
