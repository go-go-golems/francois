---
Title: Yolo K3s deployment analysis, design and intern implementation guide
Ticket: FRANCOIS-003
Status: active
Topics:
    - deployment
    - kubernetes
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/catalog.go
      Note: catalog serve flags and startup validation mapped to Deployment args
    - Path: repo://deploy/compose/Dockerfile
      Note: Image to promote to the root Dockerfile for CI
    - Path: repo://pkg/catalog/identity/oidc.go
      Note: OIDC scopes; code change C1 adds ZITADEL org scope
    - Path: repo://pkg/catalog/server.go
      Note: Exact-Host dispatch that forces probe Host headers and wildcard routing
    - Path: repo://ttmp/2026/10/13/FRANCOIS-003--deploy-the-hosted-francois-catalog-to-the-scapegoat-yolo-k3s-cluster-with-argo-cd/reference/gitops-draft/gitops/kustomize/francois/deployment.yaml
      Note: Draft workload manifest validated with validate_gitops.sh
    - Path: repo://ttmp/2026/10/13/FRANCOIS-003--deploy-the-hosted-francois-catalog-to-the-scapegoat-yolo-k3s-cluster-with-argo-cd/reference/terraform-draft/zitadel/francois/envs/prod/main.tf
      Note: Draft ZITADEL org, project and PKCE client
ExternalSources: []
Summary: How to run the hosted Francois catalog on the single-node Hetzner K3s cluster (*.yolo.scapegoat.dev) through Argo CD, Vault/VSO, cert-manager DNS-01, the shared PostgreSQL, the production ZITADEL instance and a Stripe sandbox; evidence, gaps, manifests, decisions and a phased runbook.
LastUpdated: 2026-10-13T13:30:00-04:00
WhatFor: Orient a new engineer and give them a complete, ordered plan for the first yolo deployment of Francois.
WhenToUse: Before writing the GitOps package, the Terraform environments, the CI workflow or the Vault bootstrap script for Francois.
---

# Deploying the hosted Francois catalog to the scapegoat yolo K3s cluster

This guide explains how to run the hosted Francois catalog (built in FRANCOIS-002) on the cluster that serves `*.yolo.scapegoat.dev`. It is written for an engineer who knows neither system. The first three sections explain what is being deployed, what the cluster is and which words the rest of the guide uses. The middle sections compare what Francois needs with what the platform provides, list the gaps, and give the complete design, including every manifest. The last sections give a phased runbook, a test plan, the open risks and a reference appendix.

Nothing in this guide has been applied to the cluster yet. It is a design and an implementation plan. Three facts in it were verified while writing it (Section 4.6). Everything else is tied to files in three repositories:

| Short name | Path | Role |
| --- | --- | --- |
| francois repo | `/home/manuel/code/wesen/go-go-golems/francois` | Application source, Dockerfile, CI caller workflow |
| k3s repo | `/home/manuel/code/wesen/2026-03-27--hetzner-k3s` | GitOps manifests, Argo CD Applications, Vault policies and roles, bootstrap scripts |
| terraform repo | `/home/manuel/code/wesen/terraform` | DNS records, ZITADEL organizations and OIDC clients, Stripe catalog |

## 1. Executive summary

Francois Catalog is a single Go process. It serves a trusted portal at one origin and runs subscriber application instances on separate origins, one per instance, of the form `i-<id>.<app-domain>`. It stores everything durable in PostgreSQL plus a directory of content-addressed release files. It needs four external services: an OIDC provider (ZITADEL), Stripe, an optional Pi model worker for metered LLM calls, and wildcard DNS and TLS for the instance origins.

The yolo cluster already provides each of these as a reusable platform pattern. The one non-routine choice is the hostname layout:

- **Portal:** `https://francois.yolo.scapegoat.dev`.
- **Instance origins:** `https://i-<id>.apps.francois.yolo.scapegoat.dev`. These names already resolve to the cluster through the existing `*.yolo` record (verified with `dig`, Section 4.6); an explicit `*.apps.francois.yolo` record is still recommended so the intent is declared in Terraform. They need a wildcard certificate. The certificate is issued by the existing `letsencrypt-prod-dns01-digitalocean` ClusterIssuer, which is already used the same way for go-go-host (`*.hosting.yolo.scapegoat.dev`).
- **Identity:** a new `francois` organization, project and public PKCE web client on the production ZITADEL instance at `https://zitadel.yolo.scapegoat.dev`, managed by a new Terraform environment `terraform/zitadel/francois/envs/prod`. It follows the Datadrop environment.
- **Database:** a `francois` database and role on the shared cluster PostgreSQL 16, created by the standard Vault-backed bootstrap Job (the hair-booking and go-go-host pattern).
- **Secrets:** Vault KV paths under `kv/apps/francois/prod/`, rendered into Kubernetes Secrets by Vault Secrets Operator (VSO) and mounted as files. Francois already reads every secret from a file (`--database-url-file`, `--stripe-secret-key-file`, and so on).
- **Workload:** one replica, `strategy: Recreate`, and one `local-path` PVC for release blobs and the model worker's private directory. The PVC and the Deployment are in the same sync wave.
- **Delivery:** a GHCR image published by the shared `infra-tooling` workflow, which opens an image-pin PR against the k3s repo. Argo CD (project `prod-apps`) reconciles it.

The investigation found six gaps that must be closed before or during the first deployment. Section 5 describes each in detail.

1. **The francois repo has no Git remote, no production Dockerfile at the root and no image-publishing workflow.** Nothing can be built by CI yet.
2. **Kubernetes health probes would fail as currently written.** Francois dispatches requests by exact `Host` header. A kubelet probe sends `Host: <pod-ip>:<port>`, which Francois answers with HTTP 421 (verified locally, Section 4.6). The probes must send `Host: francois.yolo.scapegoat.dev`.
3. **Francois cannot yet request a ZITADEL organization scope.** It asks only for `openid profile email` (`pkg/catalog/identity/oidc.go:51`). Without `urn:zitadel:iam:org:id:<id>`, self-registered users land in the instance's default organization rather than a dedicated Francois organization. A small flag is needed.
4. **The Stripe webhook endpoint and its signing secret do not exist for a public URL.** Local testing used the Stripe CLI forwarder. The Terraform Stripe tree deliberately does not create webhook endpoints (`terraform/stripe/README.md`).
5. **The Compose entrypoint scripts are not suitable for Kubernetes.** `deploy/compose/bin/catalog-serve` binds `127.0.0.1` and reads `/bootstrap`. The Deployment must call `francois catalog serve` directly with cluster paths, plus an init step that copies the Pi provider files into a writable directory.
6. **Release blobs on the PVC are not backed up.** The cluster's nightly `pg_dumpall` covers the database, but the `releases/blobs/` directory is not covered. Without it, a restored database points to files that no longer exist.

The deployment is split into nine phases (Section 10), each with a validation gate. The minimum path to a working beta is phases 0 through 7. The beta runs with Stripe in sandbox mode, the fake model backend first and the Pi backend second.

## 2. Problem statement and scope

### 2.1 The request

The request is to deploy Francois to the scapegoat yolo Argo CD cluster: a K3s cluster on Hetzner, defined by the k3s repo, with shared identity and DNS defined by the terraform repo. Francois must run there as a real, reachable service with sign-in, billing, instance launch and model calls. No code was written for Kubernetes during FRANCOIS-002. The only full-stack environment so far is the local Docker Compose stack in `deploy/compose/`.

### 2.2 In scope

- The hostname, DNS and TLS layout for the portal and the instance origins.
- ZITADEL configuration for subscriber sign-in, including self-registration.
- Stripe sandbox configuration for a public URL: Price, webhook endpoint and signing secret.
- The Vault secret contract, Vault Kubernetes-auth roles and the VSO resources.
- The PostgreSQL database bootstrap.
- The Kubernetes package (Kustomize) and the Argo CD `Application`.
- The image build and the automated GitOps image-pin pull request.
- The small Francois code changes the deployment needs.
- Validation, operations (first administrator, settings, publishing), backup and rollback.

### 2.3 Out of scope

- Live-mode Stripe. The guide keeps the live path open (Section 7.3, Section 12) but the first deployment uses the sandbox.
- Horizontal scaling. Francois keeps several pieces of state in process memory (Section 4.4), so it is a single-replica service by design.
- A separate registrable domain for instance origins. This is recorded as a production hardening decision (Decision D1) but is not needed for the beta.
- Email notifications (FRANCOIS-002 Phase 7), which are not implemented in Francois yet.

## 3. Orientation

This section explains the two systems that meet in this deployment and the vocabulary used for the rest of the guide. Read it before Section 4.

### 3.1 What Francois Catalog is

Francois began as a local tool that runs Claude-style HTML artifacts on a developer's machine with access to a Pi model runtime. FRANCOIS-002 turned it into a hosted subscriber platform. The hosted platform is started with `francois catalog serve` and has the following parts, all in one process:

- **Portal.** The trusted origin (`--public-url`). It serves the React subscriber catalog, the account and billing pages, the instance launcher and the administrator screens. It also serves the JSON API under `/api/`, the OIDC login under `/auth/`, the Stripe webhook receiver at `/webhooks/stripe` and the probes `/healthz` and `/readyz`.
- **Instances.** A subscriber launches a *release* (an immutable, admin-published HTML application) as an *instance*. Each instance gets its own browser origin, `i-<opaque-id>.<app-domain>`, so that untrusted application code cannot read the portal's cookies or DOM. The same Go process serves those origins. It recognizes them by the `Host` header.
- **Launch grants.** The portal embeds an instance in an iframe and hands it a short-lived, signed *runtime grant* through a `postMessage` handshake. The instance's HTTP calls (document storage, shared rooms, assets, model calls) carry that grant. Each call is re-authorized on the server.
- **Model calls.** Instances can call approved LLM models through a Pi worker: a Node.js child process started by Francois (`worker/dist/main.js`). Calls are charged against a per-user monthly credit allowance held in PostgreSQL. The `fake` backend returns deterministic text and needs no credentials.
- **Billing.** Stripe Checkout creates a subscription. Stripe webhooks are verified, stored and processed by a background worker that reconciles each user's access. Administrators can also grant complimentary access.

The companion report in the go-go-parc vault (`Projects/2026/10/13/PROJECT REPORT - Francois Catalog - … A Technical Deep Dive.md`) explains these parts in depth. This guide relies only on the facts listed in Section 4.

### 3.2 What the yolo platform is

The yolo platform is a single Hetzner virtual machine running K3s, a lightweight Kubernetes distribution. Everything on it is declared in Git and applied by Argo CD (`docs/cluster-architecture-overview.md` in the k3s repo). The pieces Francois touches are:

- **Argo CD.** Watches the k3s repo. Every application has an `Application` object (`gitops/applications/<app>.yaml`) that points at a Kustomize directory (`gitops/kustomize/<app>/`). Each `Application` belongs to an `AppProject` (`gitops/projects/*.yaml`), and the project limits which namespaces it may deploy into. There is no app-of-apps, so a new `Application` must be applied once by hand (`docs/app-deployment-pipeline.md`, "The Most Common Misunderstanding").
- **Sync waves.** The annotation `argocd.argoproj.io/sync-wave` orders resource creation. Argo CD waits for each wave to become healthy before starting the next.
- **Traefik.** The ingress controller bundled with K3s. It terminates TLS and forwards requests to Services unchanged, including the `Host` header.
- **cert-manager.** Issues Let's Encrypt certificates. Two ClusterIssuers exist (`gitops/kustomize/platform-cert-issuer/`). `letsencrypt-prod` uses HTTP-01 and can issue only exact names. `letsencrypt-prod-dns01-digitalocean` uses DNS-01 through the DigitalOcean API and can issue wildcard names.
- **Vault and VSO.** HashiCorp Vault stores all secret values. The Vault Secrets Operator reads `VaultConnection`, `VaultAuth` and `VaultStaticSecret` resources from Git and writes ordinary Kubernetes `Secret` objects. Pods authenticate to Vault through the Kubernetes auth method, using a role bound to their ServiceAccount (`vault/roles/kubernetes/*.json`) and a policy (`vault/policies/kubernetes/*.hcl`).
- **Shared PostgreSQL.** One PostgreSQL 16 StatefulSet in namespace `postgres` (`gitops/kustomize/postgres/statefulset.yaml:42`, image `postgres:16-alpine`), reachable at `postgres.postgres.svc.cluster.local:5432`. Each application gets its own database and role, created by a bootstrap Job (`docs/vault-backed-postgres-bootstrap-job-pattern.md`). A nightly CronJob runs `pg_dumpall` (`gitops/kustomize/postgres/backup-cronjob.yaml:36`).
- **local-path storage.** The default StorageClass. A PVC binds only when the first Pod that uses it is scheduled (`WaitForFirstConsumer`). This is the reason for the PVC sync-wave rule in Section 7.5.
- **ZITADEL.** The production identity provider at `https://zitadel.yolo.scapegoat.dev`, deployed by the Helm-based `Application` `gitops/applications/zitadel.yaml` (chart 10.0.4, image v4.16.1, Login V2 required). Applications get their own organization, project and OIDC client through Terraform in `terraform/zitadel/<app>/envs/prod`.
- **The release pipeline.** A source repo publishes an immutable image `ghcr.io/<owner>/<repo>:sha-<short>` using the reusable workflow `go-go-golems/infra-tooling/.github/workflows/publish-ghcr-image.yml`. The same workflow opens a pull request against the k3s repo that changes exactly one `image:` line. Merging that pull request deploys the new image.

### 3.3 Terminology

| Term | Meaning in this guide |
| --- | --- |
| Portal origin | `https://francois.yolo.scapegoat.dev`, the value of `--public-url`. The trusted site. |
| App domain | `apps.francois.yolo.scapegoat.dev`, the value of `--app-domain`. |
| Instance origin | `https://i-<id>.apps.francois.yolo.scapegoat.dev`, one per instance. Untrusted HTML runs here. |
| Release | An immutable, admin-published HTML application version stored as content-addressed blobs. |
| Runtime grant | Short-lived signed token that lets one instance page call the hosted runtime for one user. |
| Issuer | The OIDC issuer URL, `https://zitadel.yolo.scapegoat.dev`. Francois compares it exactly with the `iss` claim. |
| PKCE public client | An OIDC client with no secret that proves possession of a per-login code verifier (S256). |
| VSO | Vault Secrets Operator: copies a Vault KV path into a Kubernetes Secret. |
| Sync wave | Argo CD ordering annotation. Lower waves are applied and become healthy first. |
| DNS-01 | ACME challenge proven by a DNS TXT record. Required for wildcard certificates. |
| GitOps PR | Pull request against the k3s repo that pins a new image tag. |

### 3.4 The target in one picture

```text
                         Internet (browser, Stripe, ZITADEL redirects)
                                         |
                         DNS: *.yolo.scapegoat.dev        -> 91.98.46.169
                              *.apps.francois.yolo...     -> 91.98.46.169 (resolves today via *.yolo;
                                                         explicit record recommended)
                                         |
                   +---------------------v----------------------+
                   | Traefik (kube-system), TLS termination      |
                   |  francois.yolo.scapegoat.dev    cert: francois-tls (wildcard + apex)
                   |  *.apps.francois.yolo.scapegoat.dev        |
                   +---------------------+----------------------+
                                         | Host header preserved
                   namespace francois    v
   +---------------------------------------------------------------------+
   | Service francois:80 -> Pod francois (replicas 1, Recreate)           |
   |   container francois:  francois catalog serve --listen 0.0.0.0:8080   |
   |     Host == portal      -> portal mux (UI, /api, /auth, /webhooks)   |
   |     Host == i-*.apps... -> hosted runtime for that instance          |
   |     child process: node worker/dist/main.js (Pi worker, optional)    |
   |   volumes:                                                          |
   |     /var/lib/francois         <- PVC francois-data (local-path)     |
   |     /run/secrets/francois     <- Secret francois-runtime (VSO)      |
   |     /etc/francois/pi          <- Secret francois-pi (VSO, optional) |
   +------------------+---------------------------+----------------------+
                      |                           |
         postgres.postgres.svc:5432        https egress
         database "francois"               zitadel.yolo.scapegoat.dev (OIDC)
         (created by bootstrap Job)        api.stripe.com (Checkout, Portal)
                                           model provider (Pi backend)

   Vault (namespace vault)  --VSO-->  Secrets francois-runtime, francois-pi,
                                      francois-postgres-admin (bootstrap only)
   Argo CD  <-- watches --  k3s repo gitops/kustomize/francois + applications/francois.yaml
   GHCR     <-- publishes -- francois repo CI (infra-tooling) --opens PR--> k3s repo
```

## 4. Current-state analysis

This section lists what Francois requires at runtime, as the code shows it. The design in Section 7 has to satisfy each item. Paths are relative to the francois repo unless they say otherwise.

### 4.1 Process contract: the `catalog serve` command

`francois catalog serve` is defined in `cmd/francois/cmds/catalog.go` (settings struct at lines 70–92, `Run` at line 94, flags from line 446). The flags that matter for deployment are:

| Flag | Default | Cluster value | Notes |
| --- | --- | --- | --- |
| `--listen` | `127.0.0.1:8090` | `0.0.0.0:8080` | The default accepts only loopback connections. |
| `--public-url` | `http://localhost:8090` | `https://francois.yolo.scapegoat.dev` | Must be a bare origin (line 101). Its scheme turns on Secure, `__Host-` cookies and live-mode checks. |
| `--app-domain` | `apps.localhost` | `apps.francois.yolo.scapegoat.dev` | Must differ from the portal host (line 104). |
| `--database-url-file` | — | `/run/secrets/francois/database-url` | File contents take precedence over `--database-url` (`secret()` helper). |
| `--migrate` | `false` | `true` | Applies pending migrations before serving. Without it, `serve` refuses to start if any migration is pending (line 125). |
| `--release-dir` | `./var/catalog/releases` | `/var/lib/francois/releases` | Content-addressed blobs (`pkg/catalog/releases/releases.go:24`). |
| `--worker-dir` | `./var/catalog/worker` | `/var/lib/francois/worker` | Working directory of the Pi worker. |
| `--oidc-issuer` | — | `https://zitadel.yolo.scapegoat.dev` | Compared exactly with the token's `iss` (`pkg/catalog/identity/oidc.go:105`). |
| `--oidc-client-id-file` | — | `/run/secrets/francois/oidc-client-id` | Public PKCE client; no secret file is needed. |
| `--stripe-secret-key-file` | — | `/run/secrets/francois/stripe-secret-key` | Billing is off when this is empty. |
| `--stripe-webhook-secret-files` | — | `/run/secrets/francois/stripe-webhook-secret` (+ `-previous`) | A list, so the secret can be rotated without downtime. |
| `--stripe-price-id` | — | sandbox Price ID | Required whenever the Stripe key is set (line 166). |
| `--stripe-live-mode` | `false` | `false` | Must match the key's mode (line 150). |
| `--grant-ttl` | `30m` | `30m` | Between 1m and 12h (line 200). |
| `--backend` | `fake` | `fake`, then `pi` | `pi` starts the Node worker. |
| `--worker` | `$CWD/worker/dist/main.js` | `/app/worker/dist/main.js` | Path inside the image. |
| `--pi-agent-dir` | `/etc/francois/pi` | `/var/lib/francois/pi` | Must be writable (Section 4.5). |

Two properties of this command make it easy to deploy. First, every secret can be read from a file, so Kubernetes Secrets can be mounted as volumes and never appear in the Pod's environment or on its command line. Second, the command fails at startup if its configuration is inconsistent: a Stripe key without a Price, mismatched key mode, a bad public URL or pending migrations. A misconfigured rollout therefore crash-loops visibly instead of serving incorrect behavior.

### 4.2 Request routing by `Host`

`Server.Handler` (`pkg/catalog/server.go:98`) is the outermost HTTP handler. Its rule is:

```text
host := lowercase(request.Host)
if host == PublicURL.Host                 -> portal mux
else:
    split host into name and port
    if a port is present and port != AppPort -> 421 misdirected
    if no port is present but AppPort != ""  -> 421 misdirected
    if name is not exactly "<one label>." + AppDomain -> 421 misdirected
    instance := store.InstanceByHost(label)  (404 if unknown)
    hosted runtime handler for instance
```

`AppPort` defaults to the port of `--public-url`. For `https://francois.yolo.scapegoat.dev` that is the empty string. This matters in two ways for Kubernetes:

- **Traefik is compatible.** Traefik forwards the browser's `Host` header unchanged. A browser on port 443 sends `Host: francois.yolo.scapegoat.dev` with no port, which matches exactly.
- **Default probes are not compatible.** The kubelet sends HTTP probes to `http://<pod-ip>:<port>/path`, so the `Host` header is `<pod-ip>:8080`. That value is not the portal host and includes a port, so Francois returns 421 before reaching `/healthz`. A probe must override the header with `httpHeaders: [{name: Host, value: francois.yolo.scapegoat.dev}]`. Section 4.6 shows the measurement.

The same rule shapes TLS. The portal and every instance name must reach this Service through Traefik with valid certificates. Instance names are exactly one label below the app domain, so a single wildcard certificate `*.apps.francois.yolo.scapegoat.dev` covers all of them. TLS wildcards match exactly one label, which is all that is needed here.

### 4.3 Browser security properties that depend on the deployment

- **Cookies.** When `--public-url` is HTTPS, the session and login-flow cookies are named `__Host-francois_session` and `__Host-francois_flow` (`pkg/catalog/auth.go:24–35`), with `Secure`, `HttpOnly`, `Path=/`, `SameSite=Lax` and no `Domain`. Browsers do not let a subdomain set or overwrite a `__Host-` cookie. An instance origin therefore cannot plant a session cookie for the portal, even though both are under `scapegoat.dev`.
- **CSRF.** State-changing portal routes require an `X-CSRF-Token` header equal to the session's token and an `Origin` equal to the portal origin (`pkg/catalog/auth.go:73–115`). This works unchanged behind Traefik because Francois compares against its configured origin, not against forwarded headers.
- **Framing.** The portal sends a Content-Security-Policy whose `frame-src` allows `https://*.apps.francois.yolo.scapegoat.dev` (`pkg/catalog/routes.go:77`). Instance pages send `frame-ancestors <portal origin>` (`pkg/catalog/hosted/routes.go:40`). Both are computed from the flags, so no Traefik middleware should add or rewrite these headers.
- **Same site.** Every `*.yolo.scapegoat.dev` host has the same registrable domain, `scapegoat.dev`, so the portal, the instances and every other yolo application are *same-site*, though each is a separate *origin*. Francois's isolation relies on origins and `__Host-` cookies, not on site boundaries. Decision D1 records this trade-off.

### 4.4 State that lives in process memory

Several pieces of state exist only inside the running process:

- The HMAC key that signs asset URLs (`pkg/catalog/hosted/runtime.go:59`, 32 random bytes generated at startup).
- Live instance views, server-sent-event streams (`pkg/catalog/hosted/views.go:137`), shared-room fan-out and per-instance activity counters.
- The single Pi worker child process and its in-flight calls.

As a result, (a) more than one replica would split views and rooms and reject each other's signed asset URLs, and (b) a restart invalidates signed asset URLs, closes live streams and restarts the worker. Restart recovery is designed in: `Run` calls `Store.RecoverCalls` before listening (`pkg/catalog/server.go:182`), releasing or flagging model calls that were open when the process stopped, and instance pages renew their grants through the portal. The deployment must therefore use one replica with `strategy: Recreate`. Recreate also suits the `ReadWriteOnce` PVC.

### 4.5 Files on disk

| Path in the container | Content | Needs to persist? |
| --- | --- | --- |
| `/var/lib/francois/releases/blobs/xx/<sha256>` | Release HTML and manifests, content-addressed, written atomically. | **Yes.** The database refers to these keys. |
| `/var/lib/francois/worker` | Pi worker working directory. | No (scratch). |
| `/var/lib/francois/pi` | Pi provider configuration: `auth.json`, `models.json`, `models-store.json`, `settings.json`. The worker reads these (`worker/src/main.ts:58–65`) and may take locks or refresh OAuth tokens in them. | Yes while running. Vault holds the source copy. |
| `/etc/francois/pi` | Read-only source copy of the Pi provider files. | Comes from a Secret. |

The Compose stack copies the Pi files from a read-only mount into a private writable directory before starting (`deploy/compose/bin/catalog-serve`, lines 5–14). Kubernetes needs the same step, done in an init container (Section 7.5).

The image runs as user `francois`, uid 10001 (`deploy/compose/Dockerfile`, last stage). The Pod needs `fsGroup: 10001` so that the uid can write to the mounted PVC.

### 4.6 Facts verified while writing this guide

Three facts were measured rather than inferred. The scripts are in this ticket's `scripts/` directory.

**Probe behavior (`scripts/01-probe-host-header-check.sh`).** This builds the current tree, starts `catalog serve` with the production flags (`--public-url https://francois.yolo.scapegoat.dev --app-domain apps.francois.yolo.scapegoat.dev`, fake backend, local PostgreSQL) and sends probe-shaped requests:

```text
GET /healthz, Host 10.42.0.17:8080 (kubelet default)    -> 421
GET /readyz,  Host 10.42.0.17:8080 (kubelet default)    -> 421
GET /healthz, Host francois.yolo.scapegoat.dev          -> 200
GET /readyz,  Host francois.yolo.scapegoat.dev          -> 200
GET /,        Host i-unknown.apps.francois.yolo.scapegoat.dev -> 404
GET /,        Host a.b.apps.francois.yolo.scapegoat.dev -> 421
```

**PostgreSQL 16 compatibility (`scripts/02-postgres16-migration-check.sh`).** Development used PostgreSQL 17, but the cluster runs 16. The script starts `postgres:16-alpine` (16.8), applies both migrations (`001_core.sql`, `002_billing_usage.sql`) and runs the database-backed tests with `TEST_DATABASE_URL` set. All of them pass: `pkg/catalog`, `pkg/catalog/billing`, `pkg/catalog/payments` and `pkg/catalog/releases`.

**DNS (`scripts/03-dns-wildcard-resolution-check.sh`).** The authoritative DigitalOcean name server already answers `91.98.46.169` for both `francois.yolo.scapegoat.dev` and `i-abc.apps.francois.yolo.scapegoat.dev`. The existing `*.yolo` record (terraform `dns/zones/scapegoat-dev/envs/prod/main.tf`, `wildcard_yolo_a`) covers multi-label names in this zone.

### 4.7 What the platform conventions require

These rules come from `AGENTS.md` and `docs/app-deployment-pipeline.md` in the k3s repo. A design that breaks them will fail review or deadlock in Argo CD:

1. Each `local-path` PVC is in the same sync wave as the workload that mounts it. `scripts/validate_gitops.sh` enforces this.
2. Standard wave order: Namespace −3, ServiceAccounts and VaultAuth −2, VSO secrets −1, workload with its PVC 1–2, then Service, Ingress and NetworkPolicy last.
3. No secret material in Git. Values go to Vault through a repo-owned `scripts/bootstrap-<app>-*.sh`.
4. In-cluster VSO uses `http://vault.vault.svc.cluster.local:8200`, not the public Vault hostname.
5. The `Application` belongs to the correct project (`prod-apps`), and the project lists the namespace.
6. Images are pinned by `sha-<short>` tag. The CI-created PR changes one line.
7. Single-writer workloads use `replicas: 1` and `Recreate`.

### 4.8 Existing packages to copy

| Pattern needed | Closest existing package | What to take from it |
| --- | --- | --- |
| Postgres, VSO and bootstrap Job | `gitops/kustomize/hair-booking/` | Bootstrap ServiceAccount, VaultAuth, admin secret, script ConfigMap and Job; runtime secret. |
| Wildcard host and DNS-01 certificate | `gitops/kustomize/go-go-host/` (`certificate.yaml`, `ingress.yaml`) | `Certificate` with the apex and wildcard names; Ingress with two host rules sharing one TLS secret. |
| ZITADEL PKCE client in Terraform | `terraform/zitadel/datadrop/envs/prod/` | Organization, project, `application_v2` PKCE client, login policy with self-registration, PAT read from Vault through an ephemeral resource. |
| CI with GitHub App GitOps PRs | `go-go-golems/go-go-datadrop/.github/workflows/publish-image.yaml` plus `vault/roles/github-actions/datalab-gitops-pr.json` | `gitops_pr_token_source: github_app`, a repo-bound Vault JWT role and a policy that can read only `gitops-pr-app`. |
| Egress-limited NetworkPolicy including Stripe | `gitops/kustomize/todo-demo/network-policy.yaml` | DNS, Postgres, Traefik for the OIDC hairpin, and public 443. |
| Blob backup CronJob | `gitops/kustomize/datadrop/backup-cronjob.yaml` | Off-node archive of a PVC directory using a separate backup identity. |

## 5. Gap analysis

Each gap lists the evidence, the consequence and the fix. G1–G5 block a working beta. G6–G8 should be finished before real subscribers depend on the service.

### G1. No Git remote, no root Dockerfile, no image workflow

- **Evidence.** `git remote -v` in the francois repo prints nothing. The only Dockerfile is `deploy/compose/Dockerfile`. `.github/workflows/` contains lint, test, CodeQL, scanning and a goreleaser `release.yaml`, but no `publish-image.yaml`. There is no `deploy/gitops-targets.json`.
- **Consequence.** CI cannot build an image, so the cluster has nothing to pull.
- **Fix.** Create the GitHub repository whose name matches the module path (`github.com/go-go-golems/francois`) and push `main`. Add a production `Dockerfile` at the root, based on the Compose one, with OCI labels. Add `deploy/gitops-targets.json` and a caller workflow for `infra-tooling` in GitHub App mode (Section 7.7).

### G2. Probes return 421

- **Evidence.** Section 4.2 and the measurement in Section 4.6.
- **Consequence.** With plain `httpGet` probes the Pod never becomes Ready, the Service has no endpoints and Argo CD reports the application as Progressing and then Degraded.
- **Fix (no code change).** Add `httpHeaders` with `Host: francois.yolo.scapegoat.dev` to the readiness, liveness and startup probes. An alternative code change, answering `/healthz` for any Host, was rejected because it would add a route on every instance origin (Decision D7).

### G3. No ZITADEL organization scope

- **Evidence.** `pkg/catalog/identity/oidc.go:51` requests exactly `openid`, `profile` and `email`. ZITADEL's onboarding guide states that, without `urn:zitadel:iam:org:id:{id}`, the login UI applies the default organization's policies and registers new users there. hair-booking sends this scope (`scripts/bootstrap-hair-booking-zitadel-secrets.sh:35` in the k3s repo).
- **Consequence.** Subscribers who sign up would be created in the ZITADEL instance's default organization, not in a Francois organization with its own self-registration policy and branding. Francois would still work, because it keys users on issuer plus subject. However, user administration in ZITADEL would mix Francois subscribers with the instance's other users, and the Francois login policy (self-registration allowed) would not apply.
- **Fix (small code change).** Add `--oidc-extra-scopes` (a string list) to `catalog serve`, pass it through `identity.Config.Scopes` and append it to the scope list in `party()`. Deploy with `urn:zitadel:iam:org:id:<francois-org-id>`. Section 7.2 has the code sketch.

### G4. No public Stripe webhook endpoint or signing secret

- **Evidence.** The Compose stack obtains its signing secret from `stripe listen --print-secret` (`deploy/compose/compose.yaml`, service `stripe`). `terraform/stripe/README.md` says webhook endpoints are "intentionally not created by Terraform yet" because their signing secret would be stored in Terraform state. The Francois sandbox Product and Price used during development (`prod_VQpH0Crg2Ztp5I`, `price_1UPxdFLeHmcfmvuCg0zsScj5`, USD 1/month) were created by hand, not by Terraform.
- **Consequence.** Without an endpoint, subscriptions are never confirmed. Checkout completes at Stripe, but Francois never receives the events, so access is never granted.
- **Fix.** Codify the Francois Product and Price in the Stripe Terraform tree as a second module instance. Create the webhook endpoint for `https://francois.yolo.scapegoat.dev/webhooks/stripe` with the Stripe CLI or API, outside Terraform, and write its signing secret directly to Vault (Section 7.3).

### G5. Compose entrypoints assume Compose

- **Evidence.** `deploy/compose/bin/catalog-serve` hardcodes `--listen 127.0.0.1:8090`, reads `/bootstrap/*` files written by other containers and waits for the Stripe CLI. `catalog-setup` provisions ZITADEL through the management API with a bootstrap PAT and creates test users.
- **Consequence.** Neither script can be the container command in the cluster.
- **Fix.** The Deployment runs `francois catalog serve` with explicit arguments (Section 7.5). An init container does the Pi file copy that `catalog-serve` lines 5–14 perform. Terraform does the ZITADEL provisioning that `catalog-setup` performed. Test users are not created in production. The first administrator is promoted by exact issuer and subject with `francois catalog users promote` (Section 9.1).

### G6. Release blobs are not backed up

- **Evidence.** The cluster backup CronJob runs `pg_dumpall` (`gitops/kustomize/postgres/backup-cronjob.yaml`). Release bytes live under `/var/lib/francois/releases/blobs` on a `local-path` PVC on the single node.
- **Consequence.** If the node's disk is lost, the database can be restored but every release becomes unreadable: `Blobs.Read` (`pkg/catalog/releases/releases.go:80`) fails for each key. `Blobs.Missing` (line 97) already exists to detect this case.
- **Fix.** Add a backup CronJob based on Datadrop's that archives `releases/blobs` to off-node object storage. Because blobs are immutable and content-addressed, an incremental `rclone copy` (or `aws s3 sync`) is enough. A restore check can verify each `sha256` key in the database against the archive.

### G7. Single-replica limits are accepted, not removed

- **Evidence.** Section 4.4.
- **Consequence.** Each rollout is a short outage (Recreate). Each restart closes live views and invalidates signed asset URLs.
- **Fix for the beta.** Accept and document the outage. Pages recover through grant renewal. Persisting the asset-signing key in Vault would make asset URLs survive restarts (optional code change C3 in Section 7.9).

### G8. No egress policy and no resource sizing

- **Evidence.** The current pattern is `gitops/kustomize/todo-demo/network-policy.yaml`. Most packages set requests of 250m CPU and 512Mi memory with a 1Gi memory limit.
- **Consequence.** Without a policy, a compromised instance page cannot reach the cluster network directly, since it runs in the browser, but the Francois process could. That process includes the Node worker, which talks to model providers. Adding the NetworkPolicy later is easy, but it must allow the OIDC hairpin through Traefik, Stripe and the model provider.
- **Fix.** Ship the NetworkPolicy in phase 6 after a working baseline, so a blocked connection is not confused with a configuration error. Sizing: 250m CPU and 512Mi memory requested, 1536Mi memory limit to leave room for the Node worker.

## 6. Proposed architecture

### 6.1 Ownership across the three repositories

The deployment touches three repositories and two external accounts. Each piece of configuration has exactly one owner:

```text
francois repo (go-go-golems/francois)          owns: code, Dockerfile, CI caller,
  Dockerfile                                           deploy/gitops-targets.json,
  .github/workflows/publish-image.yaml                 the --oidc-extra-scopes change
  deploy/gitops-targets.json
        |
        | push to main -> infra-tooling publish-ghcr-image.yml
        |   test, build, push ghcr.io/go-go-golems/francois:sha-<short>
        |   Vault JWT login (role francois-gitops-pr) -> GitHub App token
        v
k3s repo (wesen/2026-03-27--hetzner-k3s)       owns: desired runtime state
  gitops/applications/francois.yaml                    (namespace, workload, routes,
  gitops/kustomize/francois/*                          secret wiring, image pin),
  gitops/projects/prod-apps.yaml (+ namespace)         Vault policies/roles,
  vault/policies|roles/kubernetes/francois-*           bootstrap script
  vault/policies|roles/github-actions/francois-gitops-pr
  scripts/bootstrap-francois-secrets.sh
        |
        | Argo CD sync (automated, prune, selfHeal)
        v
cluster: namespace francois  <-- VSO <-- Vault kv/apps/francois/prod/{runtime,pi}

terraform repo (wesen/terraform)                owns: shared identity, DNS, catalog
  zitadel/francois/envs/prod      -> org, project, PKCE client, login policy
  dns/zones/scapegoat-dev/envs/prod -> *.apps.francois.yolo A record (declarative)
  stripe/apps/francois/envs/sandbox -> Product + monthly Price (IDs only)

Stripe sandbox (dashboard/CLI)                  owns: webhook endpoint + signing secret
                                                      (secret written straight to Vault)
```

### 6.2 Runtime data flow for one subscriber

The following trace shows which component handles each step for a first-time subscriber, Alice, after deployment. It is the same flow that was validated in the Compose stack. Only the hostnames and the secret delivery are different.

1. Alice opens `https://francois.yolo.scapegoat.dev/`. DNS returns `91.98.46.169`. Traefik serves the `francois-tls` certificate and forwards the request to Service `francois`. Francois sees `Host: francois.yolo.scapegoat.dev` and serves the portal.
2. She clicks *Sign in*. Francois sets the `__Host-francois_flow` cookie and redirects her to `https://zitadel.yolo.scapegoat.dev/oauth/v2/authorize?...&scope=openid profile email urn:zitadel:iam:org:id:<org>&code_challenge=...`. ZITADEL's Login V2 UI applies the Francois organization's login policy, so she can register.
3. ZITADEL redirects her to `https://francois.yolo.scapegoat.dev/auth/callback?code=...`. Francois redeems the code with the PKCE verifier. This is a server-to-server request from the Pod to `zitadel.yolo.scapegoat.dev`, which resolves to the public IP and hairpins to Traefik. Francois verifies the ID token, upserts the user by issuer and subject, and sets `__Host-francois_session`.
4. She subscribes. Francois creates a Stripe Checkout Session with the server-controlled Price and redirects her to Stripe. Stripe later calls `POST https://francois.yolo.scapegoat.dev/webhooks/stripe`. Francois verifies the signature against the secret files, stores the event, and the background worker grants access.
5. She launches a release. Francois creates the instance `i-7k2…` and the portal frames `https://i-7k2….apps.francois.yolo.scapegoat.dev/`. Traefik matches the wildcard rule and the wildcard certificate. Francois sees an instance host and hands the request to the hosted runtime. The launch handshake delivers a runtime grant, and the instance page's model calls go to the same origin, then from the Pod to the model provider.

### 6.3 Startup sequence in the cluster

```text
Argo sync wave -3  Namespace francois
           wave -2  ServiceAccounts francois, francois-db-bootstrap
                    VaultConnection vault, VaultAuth francois, francois-db-bootstrap
           wave -1  VaultStaticSecret francois-runtime     -> Secret francois-runtime
                    VaultStaticSecret francois-pi          -> Secret francois-pi
                    VaultStaticSecret francois-postgres-admin -> Secret (bootstrap only)
           wave  0  ConfigMap francois-db-bootstrap-script
           wave  1  Job francois-db-bootstrap (Sync hook): CREATE/ALTER ROLE francois_app,
                    CREATE DATABASE francois OWNER francois_app (idempotent)
           wave  2  PVC francois-data + Deployment francois (same wave: local-path)
                    Service francois, Certificate francois-tls (DNS-01, may take minutes)
           wave  3  Ingress francois

Pod start:  init "prepare": mkdir data dirs, copy Pi files (if Secret present)
            main "francois catalog serve --migrate ...":
               open DB -> apply migrations -> refuse if any still pending
               open release blobs -> configure OIDC (lazy discovery)
               configure Stripe (fail fast on mode/price/secret mismatch)
               start Pi worker if --backend pi (fail fast if it cannot start)
               RecoverCalls -> listen 0.0.0.0:8080 -> payments worker
            startupProbe /readyz (Host header) -> Ready -> Service endpoints
```

## 7. Detailed design

### 7.1 Hostnames, DNS and TLS

**Names.** Portal `francois.yolo.scapegoat.dev`; app domain `apps.francois.yolo.scapegoat.dev`; instance hosts `i-<id>.apps.francois.yolo.scapegoat.dev`. Decision D1 explains the choice.

**DNS.** Both names already resolve (Section 4.6). For a declared record, add an entry to `local.base_records` in `terraform/dns/zones/scapegoat-dev/envs/prod/main.tf` next to `wildcard_hosting_yolo_a`:

```hcl
    wildcard_apps_francois_yolo_a = {
      type  = "A"
      name  = "*.apps.francois.yolo"
      value = "91.98.46.169"
      ttl   = 3600
    }
```

Apply this with the procedure in `terraform/dns/README.md`: authenticate to Vault, then `terraform plan` and `terraform apply`. The plan must show one addition and no other changes. The DigitalOcean token is read from Vault (`kv/infra/dns/scapegoat-dev`).

**TLS.** One `Certificate` resource, `francois-tls`, with both names, issued by `letsencrypt-prod-dns01-digitalocean` (draft `reference/gitops-draft/gitops/kustomize/francois/certificate.yaml`). The DNS-01 solver reads its DigitalOcean token from the `cert-manager/digitalocean-dns` Secret, which VSO renders from `kv/platform/cert-manager/prod/digitalocean` (`gitops/kustomize/platform-cert-issuer/`). The Ingress lists both hosts under one TLS entry with `secretName: francois-tls` and must not carry the `cert-manager.io/cluster-issuer` annotation. That annotation would make cert-manager's ingress-shim create a second, competing Certificate. go-go-host uses the same layout.

### 7.2 Identity: ZITADEL organization, client and the scope change

**Terraform.** Create `terraform/zitadel/francois/envs/prod/`, copied from the Datadrop environment. A validated draft is in this ticket at `reference/terraform-draft/zitadel/francois/envs/prod/`; `terraform validate` succeeds. It defines:

- `zitadel_organization.francois`, named *Francois*.
- `zitadel_project_v2.francois`, with all role checks off. Francois keeps roles in its own `users.role` column.
- `zitadel_application_v2.francois_portal`: a public OIDC web client using the authorization code flow with PKCE, redirect `https://francois.yolo.scapegoat.dev/auth/callback`, post-logout redirect `https://francois.yolo.scapegoat.dev/`, `dev_mode = false`, and `id_token_userinfo_assertion = true` so that the email and name claims are present.
- `zitadel_login_policy.francois`, with self-registration controlled by `var.allow_register` (default `true`).
- Outputs `organization_id`, `oidc_client_id` and `issuer`. These go into Vault (Section 7.4).

The ZITADEL administrator token is read from Vault at apply time (`kv/apps/zitadel/prod/terraform`, field `access_token`) through an ephemeral resource, so it never enters Terraform state.

**Why a public PKCE client and not a confidential one.** Francois supports both: `--oidc-client-secret-file` is optional (`cmd/francois/cmds/catalog.go`, `CatalogServeSettings.OIDCClientSecret`). The Datadrop environment documents the reason for the platform's preference: a public PKCE client keeps a client secret out of Terraform state and Vault altogether. The PKCE verifier already binds the code to the browser's login flow.

**Code change C1: `--oidc-extra-scopes`.** This is G3. The change is small and touches three files:

```go
// pkg/catalog/identity/oidc.go
type Config struct {
    Issuer       string
    ClientID     string
    ClientSecret string
    RedirectURL  string
    ExtraScopes  []string // e.g. urn:zitadel:iam:org:id:<org-id>
}

func (p *Provider) party(ctx context.Context) (rp.RelyingParty, error) {
    // ...
    scopes := append([]string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}, p.cfg.ExtraScopes...)
    party, err := rp.NewRelyingPartyOIDC(ctx, p.cfg.Issuer, p.cfg.ClientID, p.cfg.ClientSecret,
        p.cfg.RedirectURL, scopes, /* existing options unchanged */)
    // ...
}

// cmd/francois/cmds/catalog.go
OIDCExtraScopes []string `glazed:"oidc-extra-scopes"`
fields.New("oidc-extra-scopes", fields.TypeStringList,
    fields.WithHelp("Additional OIDC scopes, e.g. urn:zitadel:iam:org:id:<id> to select the ZITADEL organization"))
oidc := identity.New(identity.Config{ /* ... */, ExtraScopes: s.OIDCExtraScopes})
```

Validate that each scope is non-empty and contains no spaces. Add a unit test that builds the authorization URL against a stub discovery document and checks that the scope parameter contains the extra value. If the deployment must go ahead before C1 ships, remove the `--oidc-extra-scopes` argument from the Deployment. Everything still works, but sign-ups land in ZITADEL's default organization (G3).

**Issuer string.** The flag value must be exactly `https://zitadel.yolo.scapegoat.dev`, with no trailing slash. Francois rejects a token whose `iss` differs by even one character (`pkg/catalog/identity/oidc.go:105`).

**Logout.** Francois's logout revokes the portal session and runtime grants, but not the ZITADEL session. This is a known limitation of FRANCOIS-002. In production it means *Sign in* right after *Sign out* reuses the ZITADEL session. RP-initiated logout (redirecting to ZITADEL's end-session endpoint with `post_logout_redirect_uri`) is a follow-up, and the client already registers the post-logout URI.

### 7.3 Billing: Stripe sandbox Product, Price and webhook endpoint

**Catalog in Terraform.** The module `terraform/stripe/modules/billing-catalog` hardcodes the application slug in its metadata (`app = "the-scapegoat-dev"`) and in its lookup key (`the-scapegoat-pro-monthly-<env>`). To reuse it, add two variables whose defaults equal the current literals, so the existing sandbox and live plans stay at zero changes:

```hcl
variable "app_slug" {
  description = "Application slug stored in Stripe metadata."
  type        = string
  default     = "the-scapegoat-dev"
}

variable "lookup_key_prefix" {
  description = "Prefix of the Price lookup key."
  type        = string
  default     = "the-scapegoat-pro"
}
# metadata.app = var.app_slug
# lookup_key   = "${var.lookup_key_prefix}-monthly-${var.environment}"
```

Then add a separate root, `stripe/apps/francois/envs/sandbox/`, with its own state key `stripe/apps/francois/sandbox/terraform.tfstate`:

```hcl
module "billing_catalog" {
  source               = "../../../../modules/billing-catalog"
  environment          = "sandbox"
  app_slug             = "francois"
  lookup_key_prefix    = "francois-subscriber"
  product_name         = "Francois Subscriber"
  product_description  = "Hosted Francois applications with shared rooms and model calls"
  product_url          = "https://francois.yolo.scapegoat.dev"
  product_tax_code     = "txcd_10103000"
  monthly_amount_cents = 100   # placeholder; owner decides the real price
  currency             = "usd"
  tax_behavior         = "exclusive"
}
output "price_id" { value = module.billing_catalog.monthly_price_id }
```

The Product and Price created by hand during development (`prod_VQpH0Crg2Ztp5I`, `price_1UPxdFLeHmcfmvuCg0zsScj5`) should be imported into this root with `terraform import` or archived, so the account does not keep two Francois products. Before doing either, confirm that the sandbox key in `deploy/compose/.env` belongs to the same Stripe account the Terraform tree uses (open question Q2).

**Webhook endpoint (outside Terraform).** Stripe returns the endpoint's signing secret only once, when the endpoint is created. The Stripe tree's README rules out storing it in Terraform state, so create the endpoint with the Stripe CLI and pipe the secret straight into Vault without printing it:

```bash
export STRIPE_API_KEY=...   # sandbox secret key, from your password manager
secret=$(stripe webhook_endpoints create \
  --url https://francois.yolo.scapegoat.dev/webhooks/stripe \
  --description "francois yolo beta" \
  -d "enabled_events[]=customer.subscription.created" \
  -d "enabled_events[]=customer.subscription.updated" \
  -d "enabled_events[]=customer.subscription.deleted" \
  -d "enabled_events[]=checkout.session.completed" \
  -d "enabled_events[]=checkout.session.expired" \
  -d "enabled_events[]=invoice.paid" \
  -d "enabled_events[]=invoice.payment_failed" | jq -r .secret)
FRANCOIS_STRIPE_WEBHOOK_SECRET="$secret"   # consumed by bootstrap-francois-secrets.sh
unset secret
```

The event list is the one used by the Compose `stripe` service and handled by `pkg/catalog/payments`. Record the endpoint ID (`we_…`), which is not secret, in the diary.

**Rotation.** Francois accepts several webhook secret files and ignores empty ones (`cmd/francois/cmds/catalog.go`, the loop over `StripeWebhookFiles`). To rotate: roll the endpoint secret in the Stripe dashboard, which keeps the old secret valid for a grace period. Write the new value to `stripe-webhook-secret` and the old one to `stripe-webhook-secret-previous`. VSO's `rolloutRestartTargets` restarts the Pod. After the grace period, empty the `previous` key.

**Customer Portal.** Stripe's sandbox Customer Portal must be enabled in the dashboard once, under Settings → Billing → Customer portal. Otherwise `POST /api/billing/portal` fails at Stripe.

### 7.4 Secret contract in Vault

| Vault path (KV v2, mount `kv`) | Key | Producer | Consumer |
| --- | --- | --- | --- |
| `apps/francois/prod/runtime` | `database` = `francois` | bootstrap script | bootstrap Job |
| | `username` = `francois_app` | bootstrap script | bootstrap Job |
| | `password` (generated once, preserved) | bootstrap script | bootstrap Job |
| | `database-url` (`postgres://francois_app:…@postgres.postgres.svc.cluster.local:5432/francois?sslmode=disable`) | bootstrap script | `--database-url-file` |
| | `oidc-client-id` | Terraform output | `--oidc-client-id-file` |
| | `oidc-org-id` | Terraform output | env `FRANCOIS_ZITADEL_ORG_ID` → `--oidc-extra-scopes` |
| | `stripe-secret-key` (`sk_test_…`, or a restricted `rk_test_…`) | operator | `--stripe-secret-key-file` |
| | `stripe-webhook-secret` | Stripe CLI (Section 7.3) | `--stripe-webhook-secret-files` |
| | `stripe-webhook-secret-previous` (normally empty) | operator, during rotation | `--stripe-webhook-secret-files` |
| | `stripe-price-id` | Terraform output | env `STRIPE_PRICE_ID` → `--stripe-price-id` |
| `apps/francois/prod/pi` | `auth.json`, `models.json`, `models-store.json`, `settings.json` | operator, from a Pi agent directory | init container → `--pi-agent-dir` |
| `infra/postgres/cluster` (existing) | `service-host`, `service-port`, `postgres-user`, `postgres-password` | platform | bootstrap Job only |
| `ci/github/francois/gitops-pr-app` | `app_id`, `private_key` | operator (GitHub App) | CI workflow only |

The draft script `reference/gitops-draft/scripts/bootstrap-francois-secrets.sh` (shellcheck-clean) writes the runtime record. It preserves an existing database password unless `FORCE_ROTATE_DB=1` is set. It distinguishes a missing record from a Vault error, so a failed read cannot silently rotate the password. It refuses a live-mode Stripe key and never prints a value. With `FRANCOIS_PI_DIR` set, it also uploads the Pi provider files.

**Policies and roles.** The drafts are in `reference/gitops-draft/vault/`. `francois-prod` can read `runtime` and `pi`. `francois-db-bootstrap` can read `infra/postgres/cluster` and `runtime`. Only the bootstrap identity can read the cluster administrator password. The serving Pod cannot. The repository scripts derive policy and role names from file names (`scripts/bootstrap-vault-kubernetes-auth.sh`, `basename … .hcl`). GitHub Actions policies get a `gha-` prefix (`scripts/bootstrap-vault-github-actions-oidc.sh:33`), so the file `francois-gitops-pr.hcl` becomes the policy `gha-francois-gitops-pr` that the role names.

**What never goes into Vault for this app.** There is no OIDC client secret (public client) and no ZITADEL administrator token (Terraform-only path). There is no session-signing key either: Francois stores sessions in PostgreSQL and generates its in-memory HMAC key at startup.

### 7.5 The Kubernetes package

The complete draft package is in this ticket at `reference/gitops-draft/gitops/kustomize/francois/`, with the Application at `reference/gitops-draft/gitops/applications/francois.yaml`. It renders with `kubectl kustomize` (16 objects). It also passes the k3s repo's own `scripts/validate_gitops.sh` when copied into a scratch checkout (`scripts/04-validate-draft-gitops-package.sh`: `OK francois`, `Violations: 0`). To adopt it, copy the directory and the Application into the k3s repo and replace the placeholder image tag `sha-0000000` with the first published tag.

| File | Kind | Wave | Purpose |
| --- | --- | --- | --- |
| `namespace.yaml` | Namespace | −3 | Namespace `francois`. |
| `serviceaccount.yaml` | ServiceAccount | −2 | Serving identity, bound to Vault role `francois-prod`. |
| `db-bootstrap-serviceaccount.yaml` | ServiceAccount | −2 | Bootstrap identity, bound to Vault role `francois-db-bootstrap`. |
| `vault-connection.yaml` | VaultConnection | −2 | In-cluster Vault address. |
| `vault-auth.yaml`, `db-bootstrap-vault-auth.yaml` | VaultAuth | −2 | Kubernetes auth with the two roles. |
| `runtime-secret.yaml` | VaultStaticSecret | −1 | `runtime` → Secret `francois-runtime`; restarts the Deployment when the secret changes. |
| `pi-secret.yaml` | VaultStaticSecret | −1 | `pi` → Secret `francois-pi`, without the `_raw` key. |
| `postgres-admin-secret.yaml` | VaultStaticSecret | −1 | Cluster administrator credentials, bootstrap only. |
| `db-bootstrap-script-configmap.yaml` | ConfigMap | 0 | Idempotent role and database creation, the hair-booking script with names replaced. |
| `db-bootstrap-job.yaml` | Job (Sync hook) | 1 | Runs the script with `postgres:16-alpine`. |
| `persistentvolumeclaim.yaml` | PVC (local-path, 10Gi) | 2 | `/var/lib/francois`. Same wave as the Deployment. |
| `deployment.yaml` | Deployment | 2 | One replica, Recreate, init container, probes with a Host header. |
| `service.yaml` | Service | 2 | Port 80 → `http` (8080). |
| `certificate.yaml` | Certificate | 2 | DNS-01 wildcard plus portal name. |
| `ingress.yaml` | Ingress | 3 | Two host rules, one TLS secret. |
| `network-policy.yaml` | NetworkPolicy | 3 | Shipped in phase 6; not listed in `kustomization.yaml` at first. |

The Deployment is the only manifest specific to Francois, so it is reproduced here. Container-level `securityContext` and probe timeouts are omitted for space; the draft file has them.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: francois
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  replicas: 1
  strategy: {type: Recreate}
  selector:
    matchLabels: {app.kubernetes.io/name: francois, app.kubernetes.io/component: app}
  template:
    metadata:
      labels: {app.kubernetes.io/name: francois, app.kubernetes.io/component: app}
    spec:
      enableServiceLinks: false
      serviceAccountName: francois
      automountServiceAccountToken: false   # VSO authenticates, not the app
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        runAsGroup: 10001
        fsGroup: 10001
        seccompProfile: {type: RuntimeDefault}
      initContainers:
        - name: prepare                      # = catalog-serve lines 5-14
          image: ghcr.io/go-go-golems/francois:sha-0000000
          command:
            - /bin/sh
            - -ec
            - |
              install -d -m 0700 /var/lib/francois/pi /var/lib/francois/worker /var/lib/francois/releases
              for f in auth.json models.json models-store.json settings.json; do
                if [ -f "/etc/francois/pi/$f" ]; then
                  install -m 0600 "/etc/francois/pi/$f" "/var/lib/francois/pi/$f"
                fi
              done
          volumeMounts:
            - {name: data, mountPath: /var/lib/francois}
            - {name: pi, mountPath: /etc/francois/pi, readOnly: true}
      containers:
        - name: francois
          image: ghcr.io/go-go-golems/francois:sha-0000000
          env:   # nonsecret values that must appear inside flag strings
            - name: STRIPE_PRICE_ID
              valueFrom: {secretKeyRef: {name: francois-runtime, key: stripe-price-id}}
            - name: FRANCOIS_ZITADEL_ORG_ID
              valueFrom: {secretKeyRef: {name: francois-runtime, key: oidc-org-id}}
          args:
            - catalog
            - serve
            - --listen=0.0.0.0:8080
            - --public-url=https://francois.yolo.scapegoat.dev
            - --app-domain=apps.francois.yolo.scapegoat.dev
            - --migrate
            - --database-url-file=/run/secrets/francois/database-url
            - --release-dir=/var/lib/francois/releases
            - --worker-dir=/var/lib/francois/worker
            - --oidc-issuer=https://zitadel.yolo.scapegoat.dev
            - --oidc-client-id-file=/run/secrets/francois/oidc-client-id
            - --oidc-extra-scopes=urn:zitadel:iam:org:id:$(FRANCOIS_ZITADEL_ORG_ID)  # needs C1
            - --stripe-secret-key-file=/run/secrets/francois/stripe-secret-key
            - --stripe-webhook-secret-files=/run/secrets/francois/stripe-webhook-secret
            - --stripe-webhook-secret-files=/run/secrets/francois/stripe-webhook-secret-previous
            - --stripe-price-id=$(STRIPE_PRICE_ID)
            - --grant-ttl=30m
            - --backend=fake                 # phase 7 switches to pi
            - --worker=/app/worker/dist/main.js
            - --pi-agent-dir=/var/lib/francois/pi
          ports: [{name: http, containerPort: 8080}]
          startupProbe:                      # migrations run before listening
            httpGet: {path: /readyz, port: http,
                      httpHeaders: [{name: Host, value: francois.yolo.scapegoat.dev}]}
            periodSeconds: 5
            failureThreshold: 36
          readinessProbe:
            httpGet: {path: /readyz, port: http,
                      httpHeaders: [{name: Host, value: francois.yolo.scapegoat.dev}]}
            periodSeconds: 10
          livenessProbe:
            httpGet: {path: /healthz, port: http,
                      httpHeaders: [{name: Host, value: francois.yolo.scapegoat.dev}]}
            periodSeconds: 20
          resources:
            requests: {cpu: 250m, memory: 512Mi}
            limits: {memory: 1536Mi}
          volumeMounts:
            - {name: data, mountPath: /var/lib/francois}
            - {name: runtime, mountPath: /run/secrets/francois, readOnly: true}
      volumes:
        - name: data
          persistentVolumeClaim: {claimName: francois-data}
        - name: runtime
          secret:
            secretName: francois-runtime
            defaultMode: 0440
            items:   # only the file-read keys; a missing key fails the Pod start
              - {key: database-url, path: database-url}
              - {key: oidc-client-id, path: oidc-client-id}
              - {key: stripe-secret-key, path: stripe-secret-key}
              - {key: stripe-webhook-secret, path: stripe-webhook-secret}
              - {key: stripe-webhook-secret-previous, path: stripe-webhook-secret-previous}
        - name: pi
          secret: {secretName: francois-pi, optional: true, defaultMode: 0440}
```

Some details in this manifest are easy to get wrong:

- **`$(VAR)` in `args`.** Kubernetes expands `$(STRIPE_PRICE_ID)` from the container's `env` before starting the process. This is the only way to put a value from a Secret into a flag that Francois does not read from a file. Both values are non-secret (a Price ID and an organization ID). They are kept in the runtime record so that every value Terraform produces is stored in one place.
- **Repeated list flag.** `--stripe-webhook-secret-files` is a Glazed string-list flag. Giving it twice avoids depending on comma splitting.
- **`items:` on the runtime volume.** This mounts only the five file-read keys. Without `items`, the mount would also contain the database password and the `_raw` JSON that VSO adds. With `items`, a missing key (for example a forgotten `stripe-webhook-secret-previous`) stops the Pod at start with a clear event, rather than failing later in Francois.
- **`automountServiceAccountToken: false`.** The Pod never calls the Kubernetes API. VSO uses the ServiceAccount from outside the Pod.
- **The Pi volume is optional.** The fake backend runs without a `francois-pi` Secret. If the Vault path does not exist, VSO reports an error on the VaultStaticSecret, but the Pod starts.
- **`--migrate` versus a migration Job.** See Decision D4.

### 7.6 Argo CD Application and project

The draft `gitops/applications/francois.yaml` copies the hair-booking Application. It uses project `prod-apps`, `automated` with `prune` and `selfHeal`, `CreateNamespace=true` and `ServerSideApply=true`, and carries the standard `scapegoat.dev/*` labels (`has-database: "true"`, `database-type: postgres`, `has-persistent-storage: "true"`). Add `francois` to `spec.destinations` in `gitops/projects/prod-apps.yaml`:

```yaml
    - server: https://kubernetes.default.svc
      namespace: francois
```

Merging the package does not deploy it, because there is no app-of-apps. The first rollout requires a one-time `kubectl apply -f gitops/applications/francois.yaml` (Phase 5).

### 7.7 Image build and GitOps PR automation

**Repository.** Create `github.com/go-go-golems/francois`, add it as `origin` and push `main`. The module path in `go.mod` already names this repository.

**Dockerfile.** Promote `deploy/compose/Dockerfile` to the repository root as `Dockerfile` and keep the Compose stack pointing at the root file. Changes needed:

- Add OCI labels (`org.opencontainers.image.source=https://github.com/go-go-golems/francois`, `revision`, `created`). `infra-tooling` and GHCR use `source` to link the package to the repository.
- Keep `COPY deploy/compose/bin/ /usr/local/bin/` only if Compose keeps using the image. The scripts are harmless in the cluster, which never runs them.
- Keep `USER francois` (uid 10001). The Deployment's `runAsUser`, `runAsGroup` and `fsGroup` match it.
- The build already embeds the UI (`pnpm build`) and copies `worker/dist` and `node_modules`. The image needs no network access at runtime other than its configured providers.

**Target metadata.** `deploy/gitops-targets.json`:

```json
{
  "targets": [
    {
      "name": "francois-yolo",
      "gitops_repo": "wesen/2026-03-27--hetzner-k3s",
      "gitops_branch": "main",
      "manifest_path": "gitops/kustomize/francois/deployment.yaml",
      "container_name": "francois"
    }
  ]
}
```

The `open-gitops-pr` action changes the `image:` of the container named `container_name`. The init container `prepare` uses the same image and must be kept in sync. Either confirm that the action updates every container that uses the same image repository, or give `prepare` its own target entry with `"container_name": "prepare"`. Check this in the first CI-created PR: both lines must change (open question Q4).

**Caller workflow.** `.github/workflows/publish-image.yaml`, based on Datadrop's (GitHub App mode is the current standard; `docs/app-deployment-pipeline.md` advises against `gitops_pr_token_source: vault` because the PAT it reads expires):

```yaml
name: publish-image
on:
  pull_request:
  push:
    branches: [main]
    paths-ignore: ["ttmp/**", "*.md", ".github/ISSUE_TEMPLATE/**"]
  workflow_dispatch:
permissions:
  contents: read
  packages: write
  pull-requests: write
  id-token: write
concurrency:
  group: publish-image-${{ github.ref }}
  cancel-in-progress: true
jobs:
  release:
    uses: go-go-golems/infra-tooling/.github/workflows/publish-ghcr-image.yml@main
    secrets: inherit
    with:
      dockerfile: ./Dockerfile
      build_context: .
      test_command: go test ./... -count=1
      go_version_file: go.mod
      image_name: ghcr.io/go-go-golems/francois
      platforms: linux/amd64
      gitops_target_config: deploy/gitops-targets.json
      push_image: ${{ github.event_name != 'pull_request' }}
      open_gitops_pr: ${{ github.event_name != 'pull_request' && github.ref == 'refs/heads/main' }}
      gitops_pr_token_source: github_app
      vault_role: francois-gitops-pr
      gitops_app_secret_path: kv/data/ci/github/francois/gitops-pr-app
      gitops_app_owner: wesen
      gitops_app_repositories: 2026-03-27--hetzner-k3s
      tooling_repository: go-go-golems/infra-tooling
      tooling_ref: main
```

`go test ./...` is safe on a clean runner: the database-backed tests skip themselves when `TEST_DATABASE_URL` is unset (`pkg/catalog/catalog_test.go:36–38`). The frontend checks stay in the existing `frontend.yml`.

**Vault side for CI.** The drafts are `reference/gitops-draft/vault/roles/github-actions/francois-gitops-pr.json` (bound to `repository = go-go-golems/francois`, `ref = refs/heads/main`, `event_name = push`) and `…/policies/github-actions/francois-gitops-pr.hcl` (read access to `kv/data/ci/github/francois/gitops-pr-app` only). Copy the GitHub App credential that other repositories use into that path (`app_id`, `private_key`). Check that the App is installed on `wesen/2026-03-27--hetzner-k3s` with Contents and Pull requests write. This is the failure mode called out in `docs/app-deployment-pipeline.md`, "Local dry-run works but CI PR fails".

**Image visibility.** If the GHCR package is public, the Deployment needs no pull secret. If it is private, add an image-pull VaultStaticSecret and `imagePullSecrets` exactly as hair-booking does (`image-pull-secret.yaml`, `serviceaccount.yaml`). Decision D6 covers this.

### 7.8 Model backend

The first rollout uses `--backend fake`. Every hosted feature works, and model calls return deterministic text, so the whole sign-in, billing and launch path can be verified without provider credentials. Phase 7 switches to `--backend pi`:

1. On an operator machine, prepare a Pi agent directory containing only provider configuration (`auth.json`, `models.json`, `models-store.json`, `settings.json`), not sessions or extensions. Use a provider whose credential does not need periodic interactive refresh. An API key, or LunaRoute's gateway key (whose refresh returns the same credential, `worker/src/lunaroute.ts`), is suitable.
2. Run `FRANCOIS_PI_DIR=<dir> scripts/bootstrap-francois-secrets.sh`, which writes `kv/apps/francois/prod/pi`.
3. Change `--backend=fake` to `--backend=pi` in the Deployment through a reviewed GitOps PR.
4. In the admin UI, under Settings, approve one or more models by `provider/model` selector and set the monthly allowance. The default settings document approves no models (`pkg/catalog/settings/settings.go:56`), so hosted sampling stays off until an administrator enables it.

The init container copies Vault's version over the PVC copy on every start, so Vault stays the source of truth. A credential that Pi refreshes and writes back at runtime would be lost on restart. This is why step 1 calls for non-refreshing credentials.

### 7.9 Summary of required and optional code changes in the francois repo

| ID | Change | Needed for | Size |
| --- | --- | --- | --- |
| C1 | `--oidc-extra-scopes` flag plus a test (Section 7.2) | G3, a dedicated ZITADEL organization | ~30 lines |
| C2 | Root `Dockerfile` with OCI labels, `deploy/gitops-targets.json`, `publish-image.yaml` | G1 | 3 files |
| C3 (optional) | `--asset-signing-key-file` so signed asset URLs survive restarts | G7 | ~20 lines |
| C4 (optional) | RP-initiated logout through ZITADEL's end-session endpoint | logout limitation | ~40 lines plus UI |
| C5 (optional) | Remove the no-op `bytes.ReplaceAll(boot, "<", "<")` at `pkg/catalog/hosted/routes.go:188` | correctness hygiene | 1 line |

## 8. Decision records

### Decision D1: Instance origins under `apps.francois.yolo.scapegoat.dev`

- **Context.** Francois requires an app domain separate from the portal host (`cmd/francois/cmds/catalog.go:104`). The FRANCOIS-002 design guide prefers a domain with a separate registrable domain ("Prefer a separately controlled application domain distinct from the account site's registrable domain"). The yolo cluster has DNS-01 automation only for zones hosted at DigitalOcean.
- **Options considered.** (a) `*.apps.francois.yolo.scapegoat.dev`. (b) A sibling such as `*.francois-apps.yolo.scapegoat.dev`. (c) A new registrable domain dedicated to untrusted content, with its own DigitalOcean zone and a DNS-01 solver for it.
- **Decision.** (a) for the beta.
- **Rationale.** (a) and (b) are equally same-site with the portal, since both are under `scapegoat.dev`, so (b) gains nothing. (a) keeps every Francois name under one prefix and copies the proven go-go-host layout. The protections that matter already hold under (a). Each instance is a separate origin, so it cannot read the portal's DOM or storage. The session cookie is `__Host-` prefixed, so an instance cannot set or overwrite it. CSRF checks require the exact portal `Origin`. Release HTML is uploaded only by administrators, so it is not anonymous user content.
- **Consequences.** Instance pages are same-site with every yolo application. They could set cookies with `Domain=scapegoat.dev` that other yolo applications without `__Host-` cookies would receive. Option (c) removes this before a public, non-beta launch, and needs only new DNS and certificate configuration, not code.
- **Status.** Proposed.

### Decision D2: One replica with `Recreate`

- **Context.** In-memory views, rooms, worker and HMAC key (Section 4.4). A `ReadWriteOnce` local-path PVC.
- **Options considered.** One replica with Recreate. One replica with RollingUpdate and `maxSurge: 0` (hair-booking). Several replicas with shared state.
- **Decision.** One replica, Recreate.
- **Rationale.** Recreate guarantees that two processes never serve the same instance or write the same PVC. RollingUpdate with `maxSurge: 0` behaves the same in practice but states the intent less clearly. Several replicas would require moving rooms and views into PostgreSQL or Redis: a redesign, not a deployment task.
- **Consequences.** Each rollout causes seconds of downtime while the new Pod migrates and starts. Live pages reconnect and renew their grants.
- **Status.** Proposed.

### Decision D3: Secrets as mounted files, not environment variables

- **Context.** Francois reads secrets from files with `*-file` flags. Most yolo applications inject secrets as environment variables.
- **Decision.** Mount `francois-runtime` read-only at `/run/secrets/francois` with `items`. Only the two non-secret IDs go through `env` for `$(VAR)` expansion.
- **Rationale.** Environment variables appear in `kubectl describe`, in crash dumps and in child processes. The Pi worker is a child process and does not need them. Files can be limited to the keys that are needed.
- **Consequences.** Secret updates take effect only after a restart, which `rolloutRestartTargets` triggers. This is the first package on the cluster to use `rolloutRestartTargets`; VSO chart 1.3.0 supports it. Verify it on the first rotation.
- **Status.** Proposed.

### Decision D4: Migrations with `serve --migrate`, not a separate Job

- **Context.** `serve` refuses to start if migrations are pending (line 125). hair-booking migrates on startup (`HAIR_BOOKING_AUTO_MIGRATE=true`).
- **Options considered.** `--migrate` on the serving process; an Argo PreSync Job running `francois catalog migrate`; manual migration.
- **Decision.** `--migrate`.
- **Rationale.** There is one replica, so two processes cannot migrate at once. The migrations are forward-only, transactional SQL files. A PreSync Job would need the database credentials and the image pinned in two places. If a migration fails, the Pod crash-loops and the previous version is already gone (Recreate). Rollback is then a database restore, the same as with a Job.
- **Consequences.** Before any release that contains a new migration, check that the nightly `pg_dumpall` succeeded, or take a manual dump (Section 9.4).
- **Status.** Proposed.

### Decision D5: Stripe webhook endpoint created outside Terraform

- **Context.** The endpoint's signing secret is returned only when the endpoint is created. `terraform/stripe/README.md` forbids putting it in state until an explicit decision is made.
- **Decision.** Create the endpoint with the Stripe CLI and pipe the secret into the bootstrap script. Terraform keeps only the Product and Price.
- **Consequences.** The endpoint is configuration that is not declared anywhere. Record its ID and event list in the diary, and re-create it from this guide if it is lost.
- **Status.** Proposed.

### Decision D6: Public GHCR image

- **Context.** The cluster can pull private images through VSO-rendered pull secrets. Public images need nothing.
- **Decision.** Publish `ghcr.io/go-go-golems/francois` publicly, matching an open-source repository. If the repository stays private, use the hair-booking pull-secret pattern (open question Q1).
- **Consequences.** A public image contains the embedded UI and worker code, but no credentials. All credentials arrive at runtime from Vault.
- **Status.** Proposed, pending Q1.

### Decision D7: Probe Host header instead of a host-independent health route

- **Context.** G2.
- **Options considered.** `httpHeaders` Host on each probe; a code change that answers `/healthz` for any Host; a `tcpSocket` probe.
- **Decision.** `httpHeaders`.
- **Rationale.** No code change is needed, and the probe still exercises the real portal route, including the database ping in `/readyz`. A route that answers for any Host would also exist on every instance origin. A TCP probe would not detect a lost database.
- **Status.** Proposed.

## 9. Key flows and pseudocode

### 9.1 Bootstrapping the first administrator

Francois has no default administrator. FRANCOIS-002 made promotion an operator action keyed on the exact issuer and subject, so that an email address alone cannot claim the role:

```bash
# 1. Sign in once at https://francois.yolo.scapegoat.dev with the operator's
#    ZITADEL account, or skip this step: promote pre-creates the identity.
# 2. Find the ZITADEL user ID (the subject): ZITADEL console -> Users -> the
#    user -> ID, or from the francois DB after first login:
kubectl -n francois exec deploy/francois -- \
  francois catalog users list --database-url-file /run/secrets/francois/database-url
# 3. Promote by exact issuer + subject:
kubectl -n francois exec deploy/francois -- \
  francois catalog users promote --database-url-file /run/secrets/francois/database-url \
    --issuer https://zitadel.yolo.scapegoat.dev --subject <zitadel-user-id>
# 4. Sign out and in again; the Admin link appears.
```

All CLI subcommands (`catalog users list|promote|disable|enable|revoke-sessions|grant-access`, `catalog settings`, `catalog publish`) accept `--database-url-file`. They run inside the Pod with the mounted secret, so no credential ever reaches the operator's terminal.

### 9.2 Publishing the first release

Either use the admin UI (Admin → Releases → upload HTML → check → draft → preview → publish), or run the CLI from inside the Pod. The image already contains the repository's `examples/` at `/app/examples`, so the bundled examples need no `kubectl cp`. Other HTML files can be copied in first:

```bash
kubectl -n francois cp examples/polis.html <pod>:/tmp/polis.html
kubectl -n francois cp examples/polis.manifest.json <pod>:/tmp/polis.manifest.json
kubectl -n francois exec deploy/francois -- francois catalog publish \
  --database-url-file /run/secrets/francois/database-url \
  --release-dir /var/lib/francois/releases \
  --artifact-id polis --title Polis --version 1.0.0 \
  --html /tmp/polis.html --manifest /tmp/polis.manifest.json \
  --actor-user-id <admin-user-id>
```

### 9.3 Webhook handling end to end

```text
Stripe sandbox --POST /webhooks/stripe--> Traefik --> francois portal mux
  verify Stripe-Signature against each file in --stripe-webhook-secret-files
    none matches -> 400 (Stripe retries with backoff; visible in the dashboard)
  insert event row (idempotent on event id) -> 200 quickly
payments worker (errgroup in Server.Run):
  claim pending events -> fetch authoritative Subscription from Stripe API
  -> upsert subscription -> OnChange(user) -> Access decision
  -> if access lost: Runtime.RevokeUser(user) closes live views
```

For an intern, the main diagnostic point is that an HTTP 200 from the webhook endpoint means only that the event was *stored*. Processing happens afterwards, in the worker. Admin → Billing shows events and lets an administrator retry failed ones (`POST /api/admin/billing/events/{id}/retry`) or reconcile a user (`POST /api/admin/billing/reconcile`).

### 9.4 Upgrade, rollback and restore

```text
upgrade (normal):
  merge francois PR -> CI publishes sha-<new> -> CI opens k3s PR (one or two image lines)
  reviewer checks: only image lines changed; release notes mention migrations?
  merge -> Argo syncs -> Recreate: old pod stops, new pod migrates, probes pass

rollback (no migration in between):
  revert the k3s image PR -> Argo deploys sha-<old>

rollback (a migration was applied):
  the old binary starts (it checks only for *pending* migrations) but runs
  against a schema it does not know, so restore instead:
    scale francois to 0 (Argo: disable selfHeal or sync with the revert + replicas 0)
    restore database "francois" from the pre-upgrade dump
    deploy sha-<old>
```

Francois only refuses to start when migrations are *pending*. It does not detect a schema that is newer than it knows. Because of this, always take a dump before a release with migrations:

```bash
kubectl -n postgres exec statefulset/postgres -- \
  sh -c 'pg_dump -U "$POSTGRES_USER" -Fc francois' > francois-$(date +%F).dump
```

## 10. Implementation phases (operator runbook)

Each phase ends with a validation gate. Do not start a phase until the previous gate passes. Record each phase as a diary step in this ticket.

### Phase 0: Decisions

- Answer Q1 (public or private image) and Q2 (which Stripe sandbox account).
- Confirm the hostnames in D1.
- **Gate:** answers are recorded in the diary.

### Phase 1: Source repository ready (francois repo)

1. Create `github.com/go-go-golems/francois`, `git remote add origin …`, push `main`.
2. Implement C1 (`--oidc-extra-scopes`) with its test.
3. Add the root `Dockerfile` (C2), `deploy/gitops-targets.json` and `.github/workflows/publish-image.yaml` with `open_gitops_pr: false` for now.
4. **Gate:** a `pull_request` run builds and tests. The first `main` run publishes `ghcr.io/go-go-golems/francois:sha-<short>`. Record the exact tag. `docker pull` it from a clean machine; it must not require login if the package is public.

### Phase 2: Shared infrastructure (terraform repo)

1. Copy `reference/terraform-draft/zitadel/francois/envs/prod` to `terraform/zitadel/francois/envs/prod`. Then run `vault login -method=oidc` and `AWS_PROFILE=manuel terraform init / validate / plan / apply`. Record the `organization_id` and `oidc_client_id` outputs.
2. Add the `*.apps.francois.yolo` DNS record (Section 7.1). The plan must show exactly one addition.
3. Parameterize the Stripe module (`app_slug`, `lookup_key_prefix`, defaults unchanged) and confirm that the existing sandbox plan shows no changes. Add `stripe/apps/francois/envs/sandbox`, then import or archive the hand-made Product and Price. Record the `price_id` output.
4. **Gate:** `curl -s https://zitadel.yolo.scapegoat.dev/.well-known/openid-configuration | jq .issuer` prints `"https://zitadel.yolo.scapegoat.dev"`. The ZITADEL console shows the Francois organization with the `francois-portal` client. `dig` resolves the instance names. The Stripe dashboard shows one active Francois Price.

### Phase 3: Stripe webhook endpoint

1. Enable the sandbox Customer Portal.
2. Create the webhook endpoint (Section 7.3), keeping the secret in the shell variable only.
- **Gate:** the dashboard lists the endpoint with seven events. Record the endpoint ID.

### Phase 4: Vault (k3s repo)

1. Copy `reference/gitops-draft/vault/**` into the k3s repo's `vault/` directory. Copy `reference/gitops-draft/scripts/bootstrap-francois-secrets.sh` into `scripts/`.
2. Run `export VAULT_ADDR=https://vault.yolo.scapegoat.dev; vault login -method=oidc role=operators`.
3. Run `./scripts/bootstrap-vault-kubernetes-auth.sh` and `./scripts/validate-vault-kubernetes-auth.sh`.
4. Run `./scripts/bootstrap-vault-github-actions-oidc.sh` and `./scripts/validate-vault-github-actions-oidc.sh`. Seed `kv/ci/github/francois/gitops-pr-app` with the GitHub App credential.
5. Export the inputs and run `./scripts/bootstrap-francois-secrets.sh`.
6. **Gate:** `vault kv get -format=json kv/apps/francois/prod/runtime | jq '.data.data | keys'` lists the ten keys. Do not print the values. `vault read auth/kubernetes/role/francois-prod` shows the bound ServiceAccount.

### Phase 5: GitOps package and first sync (k3s repo)

1. Copy `reference/gitops-draft/gitops/kustomize/francois/` and `…/applications/francois.yaml` into the k3s repo. Set the image tag from Phase 1 in both `image:` lines. Add the namespace to `gitops/projects/prod-apps.yaml`.
2. Run `bash scripts/validate_gitops.sh`. It must pass, and `kubectl kustomize gitops/kustomize/francois` must render 16 objects.
3. Open a PR, have it reviewed and merge it.
4. Bootstrap the Application once:
   ```bash
   export KUBECONFIG=$PWD/.cache/kubeconfig-tailnet.yaml
   kubectl apply -f gitops/projects/prod-apps.yaml      # AppProjects are applied by hand
   kubectl apply -f gitops/applications/francois.yaml
   kubectl -n argocd annotate application francois argocd.argoproj.io/refresh=hard --overwrite
   ```
   AppProjects are not managed by an Application. Apply the project change by hand, as `docs/operator-troubleshooting-faq.md:374` describes. Otherwise Argo CD rejects the destination namespace `francois`.
5. Watch the rollout:
   ```bash
   kubectl -n argocd get application francois -w
   kubectl -n francois get vaultstaticsecret,secret,job,pvc,pod,certificate,ingress
   kubectl -n francois logs job/francois-db-bootstrap
   kubectl -n francois logs deploy/francois -c francois
   kubectl -n francois describe certificate francois-tls   # DNS-01 can take minutes
   ```
6. **Gate:** the Application is `Synced` and `Healthy`. The bootstrap Job is `Complete`. The Pod is `Ready`. The Certificate is `Ready=True`. The smoke tests in Section 11.2 pass.

### Phase 6: Accounts, billing and a first release

1. Promote the first administrator (Section 9.1).
2. Publish a sample release (Section 9.2).
3. As a second account, sign up through ZITADEL, subscribe with test card `4242 4242 4242 4242`, launch the release, use a shared room, and make a model call (fake backend).
4. Add `network-policy.yaml` to `kustomization.yaml` in a separate PR and repeat step 3.
5. **Gate:** the browser checklist in Section 11.3 passes before and after the NetworkPolicy.

### Phase 7: Pi backend

1. Seed `kv/apps/francois/prod/pi` (Section 7.8).
2. Change the Deployment from `--backend=fake` to `--backend=pi` in a PR.
3. Approve models and set the allowance in Admin → Settings.
4. **Gate:** `/readyz` reports `"models": true`. An instance completes a real model call, and Admin → Usage shows the charge.

### Phase 8: Automation and backups

1. Set `open_gitops_pr` to `true` in the caller workflow. Push a trivial change and confirm that CI opens a k3s PR whose diff is only the image line or lines (Q4).
2. Add the release-blob backup CronJob (G6) and run a restore check once.
3. **Gate:** a CI-created PR is merged and rolled out without manual steps. A backup archive exists off the node.

## 11. Test strategy

### 11.1 Before deployment (offline)

| Check | Command | Status |
| --- | --- | --- |
| Draft package renders and satisfies the cluster invariants | `scripts/04-validate-draft-gitops-package.sh` | Passed while writing this guide |
| Migrations and DB tests on PostgreSQL 16 | `scripts/02-postgres16-migration-check.sh` | Passed |
| Probe Host requirement | `scripts/01-probe-host-header-check.sh` | Confirmed (421 without header, 200 with) |
| DNS resolution of the proposed names | `scripts/03-dns-wildcard-resolution-check.sh` | Resolves to 91.98.46.169 |
| ZITADEL Terraform draft | `terraform init -backend=false && terraform validate` in the draft directory | Passed |
| Bootstrap script | `bash -n` and `shellcheck` | Passed |
| Full Go suite on a clean runner (no DB) | `env -u TEST_DATABASE_URL go test ./... -count=1` | Passed |
| C1 unit test | `go test ./pkg/catalog/identity/...` | To be written |

### 11.2 Smoke tests after first sync

```bash
curl -fsS https://francois.yolo.scapegoat.dev/healthz              # {"status":"ok"}
curl -fsS https://francois.yolo.scapegoat.dev/readyz               # "status":"ready"
curl -fsS https://francois.yolo.scapegoat.dev/api/session | jq .   # loginAvailable:true, billingEnabled:true,
                                                                   # appDomain:"apps.francois.yolo.scapegoat.dev"
curl -sI https://francois.yolo.scapegoat.dev/ | grep -i content-security-policy
                                       # frame-src https://*.apps.francois.yolo.scapegoat.dev
curl -s -o /dev/null -w '%{http_code}\n' https://i-doesnotexist.apps.francois.yolo.scapegoat.dev/   # 404
openssl s_client -connect francois.yolo.scapegoat.dev:443 \
  -servername i-x.apps.francois.yolo.scapegoat.dev </dev/null 2>/dev/null \
  | openssl x509 -noout -ext subjectAltName                         # lists the wildcard
curl -s -o /dev/null -w '%{http_code}\n' -X POST \
  https://francois.yolo.scapegoat.dev/webhooks/stripe -d '{}'       # 400 invalid_signature (pkg/catalog/routes.go:364);
                                                                   # 404 would mean Stripe is not configured
```

### 11.3 Browser checklist

Use two separate browser profiles, because logout does not end the ZITADEL session:

1. Anonymous: the catalog renders and *Sign in* redirects to `zitadel.yolo.scapegoat.dev` with the Francois organization's login page.
2. Register a new account. Francois shows it as signed in without access.
3. Checkout with card 4242. Return to the portal. Within seconds, the account shows active access. Admin → Billing shows the processed events.
4. Launch a release. The iframe origin is `https://i-….apps.francois.yolo.scapegoat.dev`. Browser devtools show no CSP or mixed-content errors.
5. Share the instance with the second profile's account, and check that the shared room updates live in both profiles. These are server-sent events through Traefik.
6. Make a model call. Admin → Usage shows it.
7. Cancel the subscription in the Customer Portal (sandbox). After the period ends, or after a test clock advance, the instance is frozen for the owner and the shared member.
8. Admin: suspend and resume an instance; revoke a user's sessions; check that a live view closes.

### 11.4 Failure drills

- Delete the Pod during an open model call. The new Pod logs `Recovered open model calls`, and the page renews its grant.
- Send a webhook event while the Pod is down (Stripe retries). After start, the event is processed.
- Rotate the webhook secret using the previous-secret key (Section 7.3). No event is rejected during the grace period.

## 12. Risks, alternatives and open questions

### Risks

| Risk | Likelihood | Mitigation |
| --- | --- | --- |
| DNS-01 issuance fails (DigitalOcean token scope, rate limits) | Low (go-go-host uses the same issuer) | `kubectl describe certificate/challenge`. The portal name alone can fall back to HTTP-01 for debugging. |
| Single node: disk loss | Medium over time | Nightly `pg_dumpall` plus the blob backup (G6). Restore drill in Phase 8. |
| Same-site instance origins | Accepted for the beta | D1 option (c) before a public launch. |
| CI PR updates only one of the two `image:` lines | Medium | Q4. Review the first PR. Use a second target entry if needed. |
| Pi provider credential refresh lost on restart | Medium for OAuth providers | Use API-key or gateway-key providers (Section 7.8). |
| Memory pressure from the Node worker | Low to medium | 1536Mi limit. Watch with Grafana. Raise the limit before raising the allowance. |
| Rollback after a migration | Low | Pre-release dump. Restore procedure in Section 9.4. |

### Alternatives not chosen

- **Helm chart instead of Kustomize.** The cluster's application packages use Kustomize. A chart adds templating without any need for it in a single-environment app.
- **A dedicated PostgreSQL for Francois.** The shared instance with per-application databases is the platform standard and has backups.
- **Exposing the Compose stack on the node.** This would bypass Argo CD, Vault and cert-manager, and contradicts every platform convention.
- **A ZITADEL confidential client.** This would put a secret into Terraform state and Vault for no security gain over PKCE.

### Open questions

- **Q1.** Will `github.com/go-go-golems/francois` and its GHCR package be public? This decides D6 and whether a pull secret is needed.
- **Q2.** Is the Stripe sandbox key used during FRANCOIS-002 the same account as `terraform/stripe/envs/sandbox`? This decides import versus re-creation of the Francois Price.
- **Q3.** What is the real monthly price and model allowance for the beta? Both are placeholders now: USD 1 in the sandbox Price, and the allowance in the settings document.
- **Q4.** Does `open-gitops-pr` update every container that uses the image, or only `container_name`? This decides whether `prepare` needs its own target.
- **Q5.** Self-registration open, or invite-only (`allow_register = false` plus users created in ZITADEL by hand) for the beta?
- **Q6.** Is RP-initiated logout (C4) required before the beta?

## 13. API and file reference

### 13.1 Francois HTTP surface relevant to operations

| Method and path | Host | Auth | Purpose |
| --- | --- | --- | --- |
| `GET /healthz` | portal | none | Process liveness. |
| `GET /readyz` | portal | none | Database ping plus `models` flag. 503 if the database is unavailable. |
| `GET /auth/login`, `GET /auth/callback`, `POST /auth/logout` | portal | flow cookie / session | OIDC login (PKCE, nonce), logout. |
| `GET /api/session` | portal | optional | `authenticated`, `loginAvailable`, `billingEnabled`, `appDomain`, user, CSRF token, access decision. |
| `POST /webhooks/stripe` | portal | Stripe signature | Event ingest. |
| `POST /api/billing/checkout`, `/portal`, `/refresh` | portal | session + CSRF | Checkout, Customer Portal, manual refresh. |
| `GET/POST /api/instances`, `POST /api/instances/{id}/launch` | portal | session + CSRF | Instance management and the launch handshake. |
| `/api/admin/*` | portal | admin session + CSRF | Releases, users, instances, settings, billing, usage, audit (`pkg/catalog/admin.go:19–45`). |
| `/*` | `i-<id>.apps.…` | runtime grant | Hosted runtime: page bootstrap, documents, rooms (SSE), assets, model calls. |

### 13.2 Francois CLI used in operations

| Command | Use |
| --- | --- |
| `francois catalog serve` | The workload (Section 4.1). |
| `francois catalog migrate --database-url-file F` | Manual migration if `--migrate` is ever removed. |
| `francois catalog users list\|promote\|disable\|enable\|revoke-sessions\|grant-access` | Operator account actions (`cmd/francois/cmds/catalog.go`, `CatalogUserAction`). |
| `francois catalog settings [--file F]` | Show or activate the validated settings document. |
| `francois catalog publish …` | CLI release publication through the same Publisher as the admin UI. |

### 13.3 File reference

francois repo:

- `cmd/francois/cmds/catalog.go`: flags, startup validation, operator commands.
- `pkg/catalog/server.go`: `Config`, `Handler` (Host dispatch), `authorizeInstance`, `Run`.
- `pkg/catalog/routes.go`: portal routes, probes, security headers, CSP.
- `pkg/catalog/auth.go`: cookies (`__Host-`), guard, CSRF and Origin checks.
- `pkg/catalog/identity/oidc.go`: OIDC relying party (scopes, PKCE, issuer check). C1 changes this file.
- `pkg/catalog/hosted/runtime.go`, `views.go`, `routes.go`: hosted runtime, in-memory state, SSE.
- `pkg/catalog/releases/releases.go`: blob store (`OpenBlobs`, `Put`, `Read`, `Missing`).
- `pkg/catalog/store/migrations/001_core.sql`, `002_billing_usage.sql`: schema.
- `pkg/piworker/client.go`, `worker/src/main.ts`, `worker/src/lunaroute.ts`: Pi worker.
- `deploy/compose/Dockerfile`, `compose.yaml`, `bin/catalog-serve`, `bin/catalog-setup`: the local reference stack.

k3s repo:

- `AGENTS.md`, `docs/app-deployment-pipeline.md`, `docs/app-runtime-secrets-and-identity-provisioning-playbook.md`, `docs/go-go-host-beta-deployment-playbook.md`, `docs/vault-backed-postgres-bootstrap-job-pattern.md`, `docs/operator-troubleshooting-faq.md`.
- `gitops/kustomize/hair-booking/`, `gitops/kustomize/go-go-host/`, `gitops/kustomize/datadrop/`, `gitops/kustomize/todo-demo/network-policy.yaml`, `gitops/kustomize/platform-cert-issuer/`.
- `gitops/applications/hair-booking.yaml`, `gitops/applications/zitadel.yaml`, `gitops/projects/prod-apps.yaml`.
- `scripts/validate_gitops.sh`, `scripts/bootstrap-vault-kubernetes-auth.sh`, `scripts/bootstrap-vault-github-actions-oidc.sh`, `scripts/bootstrap-go-go-host-runtime-secrets.sh`.

terraform repo:

- `zitadel/datadrop/envs/prod/` (template for the Francois environment).
- `dns/zones/scapegoat-dev/envs/prod/main.tf` (`wildcard_yolo_a`, `wildcard_hosting_yolo_a`).
- `stripe/README.md`, `stripe/modules/billing-catalog/`, `stripe/envs/sandbox/`.

infra-tooling and examples:

- `/home/manuel/code/wesen/corporate-headquarters/infra-tooling/docs/platform/source-repo-to-gitops-pr.md`.
- `/home/manuel/code/wesen/go-go-golems/go-go-datadrop/.github/workflows/publish-image.yaml` (GitHub App caller).

This ticket:

- `scripts/01-probe-host-header-check.sh`, `02-postgres16-migration-check.sh`, `03-dns-wildcard-resolution-check.sh`, `04-validate-draft-gitops-package.sh`.
- `reference/gitops-draft/` (Kustomize package, Application, Vault policies and roles, bootstrap script).
- `reference/terraform-draft/zitadel/francois/envs/prod/`.

## 14. References

- FRANCOIS-002 design guide: `ttmp/2026/10/12/FRANCOIS-002--…/design-doc/01-subscription-service-analysis-design-and-intern-implementation-guide.md` (Section 5 on origins and the hosted runtime).
- FRANCOIS-002 diary: `ttmp/2026/10/12/FRANCOIS-002--…/working-note/01-subscription-service-investigation-diary.md`.
- go-go-parc vault: `Projects/2026/10/13/PROJECT REPORT - Francois Catalog - Hosted Subscriber Platform … A Technical Deep Dive.md`.
- ZITADEL, *End-user onboarding*: organization scope `urn:zitadel:iam:org:id:{id}` (copy in `go-go-datadrop/ttmp/2026/07/27/ZITADEL-008-…/sources/web/02-zitadel-end-user-onboarding.md`).
- Kubernetes documentation, *Configure Liveness, Readiness and Startup Probes*: HTTP probes and `httpHeaders` (including `Host`).
- cert-manager documentation, *ACME DNS01, DigitalOcean*.
- Vault Secrets Operator documentation, `VaultStaticSecret` (`rolloutRestartTargets`, `destination.transformation.excludeRaw`).
- Stripe documentation, *Webhook endpoints* and *Roll webhook endpoint secrets*.
