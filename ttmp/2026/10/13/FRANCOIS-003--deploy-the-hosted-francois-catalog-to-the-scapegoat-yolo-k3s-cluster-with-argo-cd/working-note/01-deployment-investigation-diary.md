---
Title: Deployment investigation diary
Ticket: FRANCOIS-003
Status: active
Topics:
    - deployment
    - kubernetes
    - backend
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://ttmp/2026/10/13/FRANCOIS-003--deploy-the-hosted-francois-catalog-to-the-scapegoat-yolo-k3s-cluster-with-argo-cd/scripts/01-probe-host-header-check.sh
      Note: Measured 421 for kubelet-default Host
    - Path: repo://ttmp/2026/10/13/FRANCOIS-003--deploy-the-hosted-francois-catalog-to-the-scapegoat-yolo-k3s-cluster-with-argo-cd/scripts/02-postgres16-migration-check.sh
      Note: Migrations and DB tests on PostgreSQL 16
    - Path: repo://ttmp/2026/10/13/FRANCOIS-003--deploy-the-hosted-francois-catalog-to-the-scapegoat-yolo-k3s-cluster-with-argo-cd/scripts/04-validate-draft-gitops-package.sh
      Note: Runs the k3s validator on the draft package
ExternalSources: []
Summary: Evidence and decisions for deploying the hosted Francois catalog to the yolo K3s cluster; design delivered, nothing applied to the cluster yet.
LastUpdated: 2026-10-13T13:45:00-04:00
WhatFor: Preserve the investigation, measurements and decisions behind the FRANCOIS-003 deployment guide.
WhenToUse: Resume the yolo deployment or review why the manifests look the way they do.
---

# Deployment investigation diary

## Step 1: Survey the platform, measure the unknowns, write the deployment guide

The user asked how to deploy the hosted Francois catalog (FRANCOIS-002) to the scapegoat yolo Argo CD cluster, and asked for a ticket with an intern-level analysis, design and implementation guide uploaded to reMarkable. I created FRANCOIS-003. I read the k3s repo's canonical docs and the closest application packages, plus the ZITADEL, DNS and Stripe Terraform trees. I mapped Francois's `catalog serve` contract onto them, then wrote the guide together with a complete draft GitOps package, Vault policies and roles, a bootstrap script and a ZITADEL Terraform environment.

Nothing was applied to the cluster, Vault, Terraform state, Stripe or GitHub. Three uncertainties were resolved by measurement rather than inference: probe behavior, PostgreSQL 16 compatibility and DNS resolution. The draft package passes the k3s repo's own validator.

### Prompt Context
**User prompt (verbatim):** "How would we deploy this to scapegoat yolo argocd ? See @~/code/wesen/2026-03-27--hetzner-k3s/ and @~/code/wesen/terraform . Create a new docmgr ticket and Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."
**Assistant interpretation:** Produce an evidence-backed deployment design and phased runbook for Francois on the yolo K3s cluster, stored as a docmgr ticket and delivered to reMarkable. Do not perform the deployment.
**Inferred user intent:** Know exactly what has to change in which repository, and in what order, so that the deployment can be carried out safely later, by the user or by an intern.

### What I did
- Read `AGENTS.md` and these k3s docs: `cluster-architecture-overview.md`, `app-deployment-pipeline.md`, `app-runtime-secrets-and-identity-provisioning-playbook.md` and `go-go-host-beta-deployment-playbook.md`.
- Read the packages `hair-booking` (Postgres plus VSO plus bootstrap Job), `go-go-host` (wildcard DNS-01 certificate and Ingress), `platform-cert-issuer` (the two ClusterIssuers), `datadrop` (ZITADEL and backups), `todo-demo/network-policy.yaml` (Stripe egress) and `zitadel` (the production identity provider: chart 10.0.4, v4.16.1, Login V2).
- Read the terraform repo: `zitadel/datadrop/envs/prod` (PKCE client template), `dns/zones/scapegoat-dev/envs/prod/main.tf` (`*.yolo`, `*.hosting.yolo`), `stripe/README.md` and the `billing-catalog` module (hardcoded app slug and lookup key).
- Read infra-tooling `source-repo-to-gitops-pr.md`, Datadrop's GitHub App caller workflow and the Vault GitHub Actions role and policy naming (`gha-` prefix, `scripts/bootstrap-vault-github-actions-oidc.sh:33`).
- Re-read in Francois: `cmd/francois/cmds/catalog.go` (flags and startup validation), `pkg/catalog/server.go` (Host dispatch), `routes.go` (probes, CSP, webhook errors), `auth.go` (`__Host-` cookies, CSRF), `identity/oidc.go` (scopes), `hosted/runtime.go` (in-memory HMAC key), `releases/releases.go`, `piworker/client.go`, `worker/src/{main,lunaroute}.ts` and `deploy/compose/*`.
- Wrote and ran the ticket scripts `01`–`04` (below), the draft package in `reference/gitops-draft/`, and the ZITADEL environment in `reference/terraform-draft/`.
- Wrote the guide (`design-doc/01-…`): 14 sections, 7 decision records, 8 gaps, 9 phases, and test, risk and reference appendices.

### Why
Francois's runtime contract differs from the platform's usual web application in four ways that are easy to miss: routing by exact `Host`, wildcard instance origins, in-memory state that forbids a second replica, and secrets read from files. Each one changes a manifest. Measuring the uncertain ones keeps the guide from presenting guesses as facts.

### What worked
- `scripts/01-probe-host-header-check.sh` built the tree and started `catalog serve` with the production `--public-url` and `--app-domain` against the Compose PostgreSQL. Output:
  ```text
  GET /healthz, Host 10.42.0.17:8080 (kubelet default)    -> 421
  GET /readyz,  Host 10.42.0.17:8080 (kubelet default)    -> 421
  GET /healthz, Host francois.yolo.scapegoat.dev          -> 200
  GET /readyz,  Host francois.yolo.scapegoat.dev          -> 200
  GET /,        Host i-unknown.apps.francois.yolo.scapegoat.dev -> 404
  GET /,        Host a.b.apps.francois.yolo.scapegoat.dev -> 421
  ```
- `scripts/02-postgres16-migration-check.sh` used `postgres:16-alpine`, reported `PostgreSQL 16.8`, applied `001_core.sql` and `002_billing_usage.sql`, and the DB tests passed: `ok pkg/catalog 0.346s`, `ok pkg/catalog/billing`, `ok pkg/catalog/payments 0.413s`, `ok pkg/catalog/releases`. These tests call `t.Skip` only when `TEST_DATABASE_URL` is unset (`pkg/catalog/catalog_test.go:36–38`), and it was set.
- `scripts/03-dns-wildcard-resolution-check.sh`: `ns1.digitalocean.com` answers `91.98.46.169` for `francois.yolo.scapegoat.dev`, `i-abc.apps.francois.yolo.scapegoat.dev` and `a.b.yolo.scapegoat.dev`.
- `scripts/04-validate-draft-gitops-package.sh` copied the k3s `gitops/` tree and validator into a scratch directory, added the draft package and ran `scripts/validate_gitops.sh`: `OK francois`, `Violations: 0`, `RESULT: PASS`, exit 0. `kubectl kustomize` renders 16 objects.
- The ZITADEL Terraform draft passed `terraform init -backend=false` and `terraform validate` ("The configuration is valid."). I removed `.terraform` and the lock file afterwards.
- `bootstrap-francois-secrets.sh` passes `bash -n` and `shellcheck`.
- `env -u TEST_DATABASE_URL go test ./... -count=1` passes, which is how a clean CI runner would run it.

### What didn't work
- My first draft of the GitHub Actions policy was named `gha-francois-gitops-pr.hcl`. The bootstrap script adds the `gha-` prefix itself, so that file would have become `gha-gha-francois-gitops-pr`. I renamed it to `francois-gitops-pr.hcl`.
- I first wrote the Stripe webhook command with repeated `--enabled-events` flags. `stripe webhook_endpoints create --help` (local CLI) lists the parameter but not whether it repeats, so I switched to the documented generic form `-d "enabled_events[]=…"`.
- The inline Deployment excerpt in the guide first used a YAML block scalar inside a flow sequence (`command: [/bin/sh, -ec, |`), which is invalid YAML. I rewrote it in block style and checked it by parsing the excerpt with PyYAML (`Deployment 20 args`).
- The first version of the bootstrap script treated any `vault kv get` failure as "no record". That could rotate the database password after a transient Vault error. It now distinguishes `No value found` from other errors.

### What I learned
- DigitalOcean's DNS synthesizes the `*.yolo` wildcard for multi-label names (`a.b.yolo…`) and still answers for `hosting.yolo` even though `*.hosting.yolo` exists. The explicit `*.apps.francois.yolo` record is therefore a declaration, not a requirement.
- AppProjects in the k3s repo are applied by hand (`docs/operator-troubleshooting-faq.md:374`). A namespace added to `prod-apps.yaml` is not live until it is applied.
- VSO 1.3.0 is installed. No package on the cluster uses `rolloutRestartTargets` or `transformation.excludeRaw` yet, so Francois would be the first.
- Francois needs no OIDC client secret and no session key in Vault, because it uses a public PKCE client and stores sessions in PostgreSQL.

### What was tricky to build
The probe problem is subtle because nothing in the manifests looks wrong. Francois's `Handler` treats any `Host` that is not the portal as an instance name. The kubelet's default `Host` is `<pod-ip>:<port>`, which has a port while `AppPort` is empty, so the request fails before it reaches `/healthz`. The symptom would be a Pod that never becomes Ready while the logs show a healthy server. I chose the `httpHeaders` Host override over a code change (D7), because a health route that answers on every Host would also exist on every instance origin.

The second subtlety is getting values that are not secret, but are produced by Terraform, into flags that Francois does not read from files: `--stripe-price-id` and the organization ID inside `--oidc-extra-scopes`. Kubernetes `$(VAR)` expansion in `args`, from `env` entries backed by the runtime Secret, keeps every Terraform output in one Vault record.

### What warrants a second pair of eyes
- D1, same-site instance origins under `scapegoat.dev`, and whether a dedicated registrable domain is needed before any public launch.
- Q4: whether `open-gitops-pr` updates the init container's `image:` as well. If it does not, a CI rollout would run a new main container with an old `prepare` image.
- Rollback after a migration (Section 9.4). Francois detects pending migrations but not a newer schema.

### What should be done in the future
- Implement C1 (`--oidc-extra-scopes`) and C2 (Dockerfile, targets, workflow), then run Phases 0–8 of the guide.
- Add the release-blob backup (G6) before real subscribers upload or buy anything.

### Code review instructions
- Start with the guide's Sections 4.2, 5 and 7.5, then `reference/gitops-draft/gitops/kustomize/francois/deployment.yaml`.
- Re-run `scripts/04-validate-draft-gitops-package.sh` and `scripts/01-probe-host-header-check.sh`. The latter needs the Compose PostgreSQL on port 5435.

### Technical details
- Ticket path: `ttmp/2026/10/13/FRANCOIS-003--deploy-the-hosted-francois-catalog-to-the-scapegoat-yolo-k3s-cluster-with-argo-cd/`.
- Francois revision inspected: `b1a8150` (clean `main`, no remote).
- Draft outputs: `reference/gitops-draft/{gitops,vault,scripts}`, `reference/terraform-draft/zitadel/francois/envs/prod`.

## Step 2: Commit and deliver to reMarkable

I committed the ticket and its drafts as `acdaa39` in the francois repo. I uploaded the guide and this diary as one bundle with a table of contents (`remarquee upload bundle … --toc-depth 2`). The result was `OK: uploaded FRANCOIS-003 Yolo K3s Deployment Guide.pdf -> /ai/2026/10/13/FRANCOIS-003`. This proves the document reached the cloud, not that it has synced to the device. `docmgr doctor --ticket FRANCOIS-003 --stale-after 30` passed before the commit.
