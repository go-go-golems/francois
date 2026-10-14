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

## Step 3: Code changes for the deployment, public repository and Terraform

The user approved the implementation and answered the open questions. I implemented the code side: sign-out that ends the ZITADEL session, the organization scope flag, invite-only gating and the root Dockerfile. I created the public GitHub repository, added the publish workflow, fixed the vulnerabilities the new CI exposed, and prepared and partly applied the Terraform changes. The cluster and Vault steps are blocked: Tailscale is logged out and the Vault token has expired, and both need an interactive login.

Sign-out was verified in a browser against the Compose stack. After **Sign out**, the next `/auth/login` stops at ZITADEL's account selection, where the account shows as "Signed out", instead of silently signing back in.

### Prompt Context
**User prompt (verbatim):** "Ok, implement. commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)

Repo is public.

Save keys in vault.

Sign up invite only

Sign out ends zitadel."
**Assistant interpretation:** Carry out the FRANCOIS-003 runbook. Answers: Q1, public repository and image; Q5, invite-only (ZITADEL `allow_register = false`, users created in the console); Q6, implement RP-initiated logout. All credentials go to Vault.
**Inferred user intent:** Have Francois actually running on yolo with production-grade identity behavior.

### What I did
- Checked credentials. `gh` is logged in as `wesen` (repo scope). The AWS `manuel` profile works. Vault returns `403` on `token lookup-self` (expired token). `tailscale status` prints `Logged out.`, so `kubectl` cannot resolve `k3s-demo-1.tail879302.ts.net`. The public-IP kubeconfig times out.
- **C1/C4/invite-only (commit `2c50123`).**
  - `identity.Config.ExtraScopes`, `ValidateScopes` and `Provider.LogoutURL`, which builds `end_session_endpoint?client_id&post_logout_redirect_uri&id_token_hint`. `Exchange` now also returns the raw ID token.
  - Migration `003_session_id_token.sql` adds `sessions.id_token`, cleared on revoke.
  - `logout` returns `{"ok":true,"redirect":…}` and the UI follows it.
  - New flags `--oidc-extra-scopes` and `--signup-allowed` (default off; Compose passes it). `/api/session` exposes `signupAvailable`.
  - Unit tests in `pkg/catalog/identity/oidc_test.go` use a stub discovery document.
- Moved `deploy/compose/Dockerfile` to `./Dockerfile` with OCI labels. Compose now builds from the root file.
- **CI (commit `44447f2`).** Added `deploy/gitops-targets.json` and `.github/workflows/publish-image.yaml` (infra-tooling, GitHub App mode), with `open_gitops_pr: false` until the Vault role and App credential exist.
- Scanned the full history for secret patterns before publishing. The only matches were the validator's own fixtures (`sk_live_abcdefghijklmnop`, `BEGIN RSA PRIVATE KEY` in test tables). Ran `gh repo create go-go-golems/francois --public --source . --push`.
- **Fail fast (commit `9dc27cc`).** `--oidc-extra-scopes` is now validated before the database connection.
- **Security (commit `72999cc`).**
  - Toolchain `go1.26.9` (go.mod and Dockerfile `golang:1.26.9`), `golang.org/x/net v0.60.0` and `x/text v0.42.0`. `govulncheck`: "No vulnerabilities found."
  - Four gosec findings got `#nosec` justifications: G124 on the dev-only non-Secure cookie, and three G705 on fixed-type embedded assets, an octet-stream attachment, and allowlisted asset types served with `nosniff`. gosec now exits 0.
- **k3s worktree** (`feat/francois-yolo` from `origin/main` in a scratch worktree; the main checkout is on someone's `fix/hk3s-0044-tune-memory-down` branch and was not touched).
  - Copied the draft package, Vault policies and roles, and the bootstrap script.
  - Replaced the init container with a shell wrapper in the main container.
  - Added the `francois` destination to `prod-apps` and a package README.
  - `validate_gitops.sh`: `OK francois`, `Violations: 0`.
- **terraform worktree** (`feat/francois-yolo`, commit `174ca7e`).
  - `zitadel/francois/envs/prod` with `allow_register = false` by default, plus a README.
  - DNS `wildcard_apps_francois_yolo_a`.
  - Stripe module variables `app_slug`, `lookup_key_prefix` and `price_nickname`. Defaults are unchanged: the existing sandbox plan shows "No changes."
  - New root `stripe/apps/francois/envs/sandbox`.
- **Stripe applied.** Imported `prod_VQpH0Crg2Ztp5I` and `price_1UPxdFLeHmcfmvuCg0zsScj5`. The plan was "0 to add, 2 to change, 0 to destroy" (metadata, name, description, URL, tax code, lookup key, nickname, `tax_behavior` from unspecified to exclusive). Applied it, and a re-plan with `-detailed-exitcode` returned 0.

### Why
- Front-channel logout (a browser redirect) is used instead of the library's `rp.EndSession`, which calls the end-session endpoint server-side (`zitadel/oidc/v3/pkg/client/client.go:106–143`). ZITADEL's session cookie lives in the browser, so the browser has to visit the endpoint for ZITADEL to clear it.
- The ID token is stored server-side with the session because `id_token_hint` lets ZITADEL end exactly that session without asking the user which one.
- Q4 was resolved by reading `infra-tooling` `origin/main` `open_gitops_pr.py`. `replace_manifest_image_text` only enters a container list when a line is exactly `containers:`, so an `initContainers:` image would never be bumped. A shell wrapper (`sh -ec '<copy>; exec francois "$@"' francois <args>`) keeps a single image line.
- Q2: the Compose sandbox key belongs to `acct_1TxEMcLeHmcfmvuC` ("Scapegoat Consulting sandbox"), and the Terraform-managed `prod_Ux95umhPXiXgXw` ("The Scapegoat Pro") is visible with that key. It is the same account, so I imported the hand-made Francois objects rather than duplicating them.

### What worked
- Browser test on Compose, after rebuilding the image:
  1. The first `/auth/login` signed in silently (an old ZITADEL session), reproducing the problem.
  2. **Sign out**, then `/auth/login`, landed on ZITADEL "Select Account" with `Ada Admin … Signed out`.
  3. Signed in with a password, then called logout by fetch: `endpoint: http://localhost:18081/oidc/v1/end_session`, params `client_id, id_token_hint, post_logout_redirect_uri`, `hintLen: 1138`, `post: http://localhost:8090/`.
  4. Signed in again silently (ZITADEL session still alive, as expected since the fetch did not follow the redirect), clicked **Sign out**, ran `/auth/login`: "Select Account" again.
- Wrapper test in the built image as uid 10001: it copied `models.json` with mode `-rw-------` and passed the arguments through to `francois catalog serve`.
- `TEST_DATABASE_URL=… go test ./... -count=1` and `golangci-lint run ./...` (0 issues) after each code change.

### What didn't work
- In the first browser run, Playwright's network list showed only `POST /auth/logout => 200` and no request to `:18081`. I suspected the redirect was not being followed. The next `/auth/login` showed ZITADEL's "Signed out" state, which proved it was: the network list does not include top-level navigations.
- `go get golang.org/x/text@v0.41.0 golang.org/x/net@v0.60.0` failed with `golang.org/x/net@v0.60.0 requires golang.org/x/text@v0.42.0`. I used `x/text@v0.42.0`.
- The first CI `Dependency Scanning` runs on `44447f2` and `9dc27cc` failed: gosec "Issues : 4", and govulncheck reported GO-2026-6629, -6617, -6613, -6612, -6611, -6610, -6608, -6607 and -6605 (x/text, x/net and the go1.26.5 stdlib). `72999cc` addresses them.
- The first `publish-image` run (`44447f2`) was cancelled by the concurrency group when `9dc27cc` was pushed.

### What I learned
- ZITADEL's v1 login UI (Compose) and Login V2 (production) both honor `id_token_hint` plus `post_logout_redirect_uri` when the URI is registered on the client, as `catalog-setup` and the Terraform client do.
- The reusable workflow sets OCI labels through metadata-action, so the Dockerfile `ARG VERSION/REVISION` defaults matter only for local builds.

### What was tricky to build
Logout for sessions created before migration 003 has no ID token. The redirect still carries `client_id`, which ZITADEL accepts, but without a hint it may ask which session to end. `TestLogoutURL` covers the hint-less form. The portal session is revoked before the redirect is computed, so a failed or missing redirect still leaves the user signed out of Francois.

### What warrants a second pair of eyes
- `tax_behavior` changed from unspecified to `exclusive` on the Francois sandbox Price. Checkout does not enable automatic tax, so the charged amount is unchanged, but a live Price needs the owner's tax decision (`terraform/stripe/README.md`).
- Storing ID tokens in `sessions.id_token`. They are signed and contain the email address. They are cleared on revoke but are kept until then.

### What should be done in the future
- Vault, ZITADEL Terraform, DNS apply, the webhook endpoint, the k3s PR and the first sync: blocked on `vault login` and `tailscale up`.

### Code review instructions
- `pkg/catalog/identity/oidc.go` (`LogoutURL`, `Exchange`), `pkg/catalog/auth.go` (`logout`, `login`), `pkg/catalog/store/sessions.go`, `ui/catalog/ui.tsx` (Header).
- Validate with `go test ./pkg/catalog/identity/` and `TEST_DATABASE_URL=… go test ./pkg/catalog/`, then the Compose browser round trip above.

### Technical details
- francois commits: `2c50123`, `44447f2`, `9dc27cc`, `72999cc`. GitHub: https://github.com/go-go-golems/francois.
- terraform branch `feat/francois-yolo` @ `174ca7e` (worktree `scratchpad/tf-francois`). The Stripe state is at `stripe/francois/sandbox/terraform.tfstate`.
- k3s branch `feat/francois-yolo` (worktree `scratchpad/k3s-francois`), not yet committed.

## Step 4: Identity and Vault provisioned; cluster access and DigitalOcean token blocked

After the user logged in to Vault and Tailscale, I provisioned everything that does not need the cluster. That covers the ZITADEL organization and client, the Vault roles, the GitHub App credential, the disabled Stripe webhook endpoint and the runtime secret record. Two new blockers appeared, both outside this deployment's code:

- The cluster node's Tailscale key had expired, and the node is still offline.
- Both DigitalOcean tokens in Vault are rejected. One is the Terraform DNS token; the other is cert-manager's DNS-01 token, which every wildcard certificate depends on.

While checking how webhooks would behave, I found and fixed a real problem. The Stripe sandbox account is shared with other applications, and Francois would have retried their events.

### Prompt Context
**User prompt (verbatim):** "done." (after running `tailscale up` and `vault login`). Later, while I was working: "is that maybe because the hosts need to reauthenticate or so?", "you should have tailscale permission now though", "ok expiry disabled, which node is that?", "do you by chance know anything about the user to login to hetzner?"
**Assistant interpretation:** Continue the runbook with the new credentials and diagnose the cluster access problem.
**Inferred user intent:** Unblock the deployment.

### What I did
- Vault: `token lookup` shows `display_name: oidc-wesen`, identity policy `admin`.
- Cluster access:
  - `kubectl` over the tailnet failed: `dial tcp 100.73.36.123:6443: i/o timeout`.
  - `tailscale status --json` showed `k3s-demo-1 online=false … keyExpiry=2026-09-25T19:59:42Z expired=true` (last seen 2026-09-25T21:00). `k3s-proxmox` and `k3s-server` had also expired.
  - The user disabled key expiry. The status is now `keyExpiry=none expired=false`, but the node is still `online=false`.
  - The public path is closed. The Hetzner firewall allows 22 and 6443 only from `admin_cidrs = ["100.40.113.156/32"]`, and our IPv4 is `72.200.191.248`, so `ssh root@91.98.46.169` timed out.
  - The VM login is `root` with SSH key only (`~/.ssh/id_ed25519.pub` matches `terraform.tfvars`). Cloud-init sets no password.
- ZITADEL: `terraform plan` showed 4 resources to add. Applied ("4 added"); re-plan `-detailed-exitcode` returned 0. Outputs: `organization_id = 394966170567966836`, `oidc_client_id = 394966177899610228`, `project_id = 394966174242177140`.
- DNS: the plan failed with `GET https://api.digitalocean.com/v2/domains/scapegoat.dev: 401 … Unable to authenticate you`. Both Vault tokens, `kv/infra/dns/scapegoat-dev` (`api_token`, updated 2026-07-29) and `kv/platform/cert-manager/prod/digitalocean` (`access-token`, updated 2026-07-18), are 71-character `dop_v1_` tokens that get **401** from `/v2/account` and from `/v2/domains/scapegoat.dev/records`.
- Vault roles: wrote only the Francois policies and roles (`francois-prod`, `francois-db-bootstrap`, `gha-francois-gitops-pr`, plus the roles `francois-prod`, `francois-db-bootstrap` and `github-actions/francois-gitops-pr`). I did not rerun the script that rewrites every application's policies. Read-back confirmed the bound ServiceAccount, namespace and claims.
- GitHub App: all `kv/ci/github/*/gitops-pr-app` entries use app `3926776`. I copied datalab's credential to `kv/ci/github/francois/gitops-pr-app`; the `private_key` SHA-256 prefix `1e14f472a43d` matches the source.
- Stripe webhook: created `we_1UQ9zfLeHmcfmvuC639e6BPZ` for `https://francois.yolo.scapegoat.dev/webhooks/stripe` with 7 events. In the same shell, piped its secret into `scripts/bootstrap-francois-secrets.sh` and set the endpoint `disabled=true`. `kv/apps/francois/prod/runtime` holds 10 keys.
- **Code fix (commit `7f1441a`): ignore foreign Stripe customers.** Customers are now tagged `francois_deployment`. The new `Gateway.CustomerDeployment` reads the tag. In `processOne`, an event whose customer has no local mapping is marked `ignored` when the tag differs from `Config.Deployment`, and retried only for our own customers. Tests: untagged and foreign customers end with no pending events; our own unmapped customer still gets `unknown_customer`.
- Image `sha-7f1441a`: all seven workflows green, including Dependency Scanning, which failed before `72999cc`. Pinned in k3s PR #339 (commit `c19b6f2`).

### Why
- The Stripe sandbox account `acct_1TxEMcLeHmcfmvuC` serves the.scapegoat.dev TODO app, the local Compose stack and now yolo. The webhook endpoint receives every subscription and invoice event in the account. Before the fix, each foreign event would have been retried 12 times with exponential backoff, making Stripe API calls each time, and then shown as a failed event in Admin → Billing.
- I created the webhook endpoint disabled so that Stripe does not accumulate failed deliveries, or warn about and auto-disable the endpoint, while the host is not serving.
- I wrote targeted Vault policies and roles to avoid the side effects of rewriting every application's policy from a feature branch.

### What worked
- The ZITADEL apply, the Vault writes and reads, the GitHub App copy, the webhook-to-Vault pipeline (no secret printed), and CI.

### What didn't work
- `kubectl get nodes`: `dial tcp 100.73.36.123:6443: i/o timeout`. The node is still offline after expiry was disabled. It probably needs `tailscale up` on the node.
- `ssh root@91.98.46.169`: `Connection timed out` (firewall `admin_cidrs`).
- DNS Terraform and both DigitalOcean tokens: HTTP 401.
- My first token check used zsh-incompatible `set -- $pair` and an unquoted `?`: "no matches found". I reran it under `bash -c`.

### What I learned
- The DigitalOcean 401 is platform-wide. New DNS-01 certificates cannot be issued, and the existing wildcard certificates (`*.hosting.yolo`, `*.storybook.yolo`, `*.pds.yolo`) will fail to renew. Francois's `*.apps.francois.yolo` certificate cannot be issued until a new token is in `kv/platform/cert-manager/prod/digitalocean`.

### What was tricky to build
- Telling apart our own customer whose local mapping write is still in flight (retry) from a foreign customer (ignore). Only our deployment tags customers with its own `Config.Deployment` value, so the tag decides. Untagged customers created by Francois before this change exist only in the local Compose deployment, where they are mapped and never reach this branch.

### What warrants a second pair of eyes
- The Stripe sandbox account is shared by three deployments. A live account should be Francois-only, or keep this tag-based filter.

### What should be done in the future
- User actions: restore the node's tailnet (`tailscale up` on the node, or temporarily allow SSH from `72.200.191.248/32`). Create a new DigitalOcean API token with domain read/write scope and write it to both Vault paths.
- Then: DNS apply, merge PR #339, apply the AppProject and Application, enable the webhook endpoint, smoke tests, first admin, browser run.

### Technical details
- francois commits: `7f1441a` (code), image `ghcr.io/go-go-golems/francois:sha-7f1441a`. k3s PR #339 @ `c19b6f2`; terraform PR #34 @ `174ca7e`.

## Step 5: DigitalOcean token rotated, Francois DNS record applied

The user created a new DigitalOcean token and wrote it to both Vault paths. Both now pass the domain records API, so I applied the Francois DNS record. This was a targeted apply, because the zone has unrelated pending drift. The cluster node is still offline on the tailnet.

### Prompt Context
**User prompt (verbatim):** "where can i create a DO token or test if I have one already?" then "ok I wrote it to the vault, try again"
**Assistant interpretation:** Verify the rotated token and continue the DNS phase.
**Inferred user intent:** Unblock DNS and certificate issuance.

### What I did
- I first checked the local `doctl` contexts. `default`, `librechat` and `mento` all get `401 … Unable to authenticate you` on `/v2/account` and `/v2/domains/scapegoat.dev`. I pointed the user to https://cloud.digitalocean.com/account/api/tokens (Custom Scopes: domain read/update/create/delete).
- After the rotation, both Vault paths (updated 2026-10-13T18:09Z) return `records_api=200`.
- `terraform plan` on the DNS zone: "3 to add": `pds_yolo_a`, `wildcard_apps_francois_yolo_a` and `wildcard_pds_yolo_a`. I applied only `-target='digitalocean_record.records["wildcard_apps_francois_yolo_a"]'`: "1 added". The DigitalOcean API shows `1835311720 *.apps.francois.yolo 91.98.46.169 ttl=3600`.
- Commented the apply status and the PDS drift on terraform PR #34.

### Why
- `main.tf` says the PDS records "already exist in DigitalOcean (created out-of-band); import them before applying". A full apply could duplicate them. Fixing that drift belongs to HK3S-0031, not to this ticket.

### What didn't work
- Nothing new. The node is still `online=false`.

### What should be done in the future
- Someone should import or clean up the `pds_yolo_a` and `wildcard_pds_yolo_a` drift.
- cert-manager's `digitalocean-dns` Secret refreshes from Vault every 30 minutes (`refreshAfter: 30m`). Once the cluster is reachable, confirm that the existing wildcard certificates renew.

## Step 6: Cluster access restored, first sync, certificate and smoke tests

The user ran `tailscale up` on the node, and the tailnet kubeconfig works again. I removed the temporary SSH firewall rule the user had added, applied the AppProject and the Application, and watched the first sync.

Francois is live at https://francois.yolo.scapegoat.dev. The Application is Synced/Healthy and the wildcard certificate was issued through DNS-01 with the rotated token. All public smoke tests pass, and the Stripe webhook endpoint is enabled.

### Prompt Context
**User prompt (verbatim):** "merge k3s and you should be able to track it", "is the argocd login in the vault maybe? is there a diary or playbook or maybe in ~/code/wesen/go-go-golems/go-go-parc/Research/playbooks (iirc)", "ok i ran tailsccale up can you test?", "btw commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"
**Assistant interpretation:** Merge PR #339, find a way to do the one-time Argo bootstrap, then deploy and verify.
**Inferred user intent:** See the service running on yolo.

### What I did
- Merged k3s PR #339 (squash `0e06f98`). Confirmed that nothing auto-applies `gitops/applications` (the maillist README documents the same manual `kubectl apply`).
- Looked for Argo CD credentials:
  - No Argo entry exists among the 129 Vault KV paths.
  - The HK3S-0001 runbook reads `argocd-initial-admin-secret` with kubectl.
  - The k3s `.envrc` `ARGOCD_TOKEN` (93 chars) returns `no session information`.
  - The go-go-parc infra playbooks assume the Tailscale kubeconfig.
- Found that `hcloud --context k3s` works (server `k3s-demo-1`, firewall `k3s-demo-1-fw`). My attempt to add a temporary SSH rule was **denied by the permission classifier** ("Security Weaken"). I gave the user the commands instead, and they added the rule themselves.
- After `tailscale up` on the node: `k3s-demo-1 online=true`, and `kubectl get nodes` shows `k3s-demo-1 Ready control-plane 200d v1.34.5+k3s1`.
- Removed the temporary rule. The first `delete-rule` without `--description` failed ("the specified rule was not found"). With the matching description it succeeded, and port 22 is again allowed only from `100.40.113.156/32`.
- `kubectl diff` on `gitops/projects/prod-apps.yaml` showed only `+ namespace: francois`. Applied it, then applied `gitops/applications/francois.yaml` and hard-refreshed.
- Watched the sync, then ran the smoke tests and enabled the webhook with `stripe webhook_endpoints update we_1UQ9zfLeHmcfmvuC639e6BPZ -d disabled=false` (`status=enabled`).

### Why
- `kubectl` over Tailscale is the documented admin path. Restoring it is better than working around it through Argo CD or a code change for admin promotion.
- The temporary firewall rule had to go once Tailscale worked: the firewall is Terraform-managed (`admin_cidrs`), and a leftover rule is both a security exposure and drift.

### What worked
- First sync, after about 30 seconds:
  - VaultStaticSecrets `francois-runtime` and `francois-postgres-admin`: Synced/Healthy.
  - `job.batch/francois-db-bootstrap`: `Complete 1/1` in 5s.
  - `pvc/francois-data`: `Bound` (local-path, same wave as the Deployment).
  - `pod/francois-…`: `1/1 Running`, so the probes with the portal `Host` header pass.
  - `francois-pi`: SyncedFalse, as expected (no Vault record yet; the mount is optional).
- Pod log: `Applied migration … 001_core.sql`, `002_billing_usage.sql`, `003_session_id_token.sql`, then `Francois catalog ready address=[::]:8080 apps=*.apps.francois.yolo.scapegoat.dev portal=https://francois.yolo.scapegoat.dev`.
- Certificate: the challenges reported `Presented: true`, then `Waiting for DNS-01 challenge propagation`. `francois-tls` was `Ready=True` at about 109s, issued by Let's Encrypt `YR2`, with SANs `*.apps.francois.yolo.scapegoat.dev` and `francois.yolo.scapegoat.dev`. The Application is `Synced Healthy`.
- Smoke tests:
  ```text
  /healthz -> {"status":"ok"}
  /readyz  -> {"models":true,"status":"ready"}
  /api/session -> appDomain apps.francois.yolo.scapegoat.dev, billingEnabled true, loginAvailable true, signupAvailable false
  CSP frame-src https://*.apps.francois.yolo.scapegoat.dev; X-Frame-Options DENY
  https://i-doesnotexist.apps.francois.yolo.scapegoat.dev/ -> 404
  POST /webhooks/stripe {} -> 400 invalid_signature
  /auth/login -> 303 zitadel authorize, scope "openid profile email urn:zitadel:iam:org:id:394966170567966836",
                 redirect_uri https://francois.yolo.scapegoat.dev/auth/callback; ZITADEL -> 302 /ui/v2/login/login?authRequest=V2_…
  ```
- DNS-01 certificates across the cluster: all `Ready=True`. `static-sites/storybook-yolo-wildcard-tls` had `renewal=2026-10-07` (missed while the DigitalOcean token was dead; `notAfter=2026-11-06`). `cert-manager/digitalocean-dns` was re-synced by VSO at 18:16:38Z with the new token, so the renewal will be retried.

### What didn't work
- Denied by the classifier: `hcloud … firewall add-rule … --source-ips 72.200.191.248/32` (user decision, not retried).
- `hcloud firewall delete-rule` without `--description`: "the specified rule was not found in the ruleset of Firewall 10758703".

### What I learned
- `/readyz` reports `models: true` with the fake backend, because the fake model is listed. It means "a model backend is present", not "a provider is reachable".
- ZITADEL Login V2 accepts the organization scope and switches to that organization's login policy.

### What warrants a second pair of eyes
- The overdue storybook wildcard renewal: confirm it completes within a day.

### What should be done in the future
- Create the invited accounts (ZITADEL, Francois organization), promote the first admin, publish a release, then run the browser end-to-end test.

### Technical details
- k3s `main` @ `0e06f98`. Image `ghcr.io/go-go-golems/francois:sha-7f1441a`. Webhook `we_1UQ9zfLeHmcfmvuC639e6BPZ` enabled.

## Step 7: Invited accounts, first admin, first release and the production browser run

I created two invited test accounts in the Francois ZITADEL organization, promoted one to Francois administrator by exact issuer and subject, published Polis 1.0.0, and drove the production deployment end to end in headless Chromium. The run covered ZITADEL Login V2, Stripe Checkout in the sandbox with card 4242, the webhook pipeline, instance launch on the isolated origin, and sign-out through ZITADEL. A control run showed the silent re-login that sign-out now prevents.

### Prompt Context
**User prompt (verbatim):** (none new beyond Step 6; one tool call was rejected and then explained by the user: "sorry, i clicked the wrong thing")
**Assistant interpretation:** Continue Phase 6: accounts, administrator, release, and the browser checklist.
**Inferred user intent:** Proof that the deployed service works for a real subscriber.

### What I did
- **Users.** Created `e2e-admin@francois.test` (`394969329885511796`) and `e2e-alice@francois.test` (`394969332167213172`) with ZITADEL `POST /v2/users/human` in organization `394966170567966836`, verified email and random passwords. The token was the Terraform administrator token from `kv/apps/zitadel/prod/terraform`. The passwords went straight to `kv/apps/francois/prod/e2e-users` and were never printed. My first attempt was rejected by the user's click; I reran it after "sorry, i clicked the wrong thing".
- **First admin.** Ran `kubectl -n francois exec deploy/francois -- francois catalog users promote --database-url-file … --issuer https://zitadel.yolo.scapegoat.dev --subject 394969329885511796`, which returned `usr_305b270c89b81c62ee327f2a promote ok`.
- **Release.** Ran `francois catalog publish … --artifact-id polis --version 1.0.0 --html /app/examples/polis.html --manifest /app/examples/polis.manifest.json`. Result: `rel_02fefc881efb42bfc6a002cb published`, html sha256 `67b19d5e…`. `/api/catalog` lists `polis 1.0.0`.
- **E2E script.** Wrote `scripts/05-yolo-e2e.{sh,mjs}`. It uses playwright-core 1.61.1 from the npx cache and the local chromium-1223, and reads credentials from Vault inside its own process. Screenshots go to `various/e2e/`; browser state files are gitignored.

### Why
- Sign-up is invite-only, so an operator has to create accounts. The ZITADEL API with the existing Terraform token is the operator path that needs no console clicks. Keeping the passwords in Vault follows the user's "save keys in vault".
- I drove the browser from a script instead of the MCP browser so the test passwords never appear in tool calls or the transcript.

### What worked
```text
[admin] session {"authenticated":true,"role":"admin","email":"e2e-admin@francois.test","access":"subscription_required"}
[admin] admin page title: Operations
[alice-subscribe] before {"role":"subscriber","allowed":false,"reason":"subscription_required"}
[alice-subscribe] returned to /account?checkout=cs_test_…
[alice-subscribe] after {"allowed":true,"reason":"subscriber"}
[alice-run] instance origin https://i-….apps.francois.yolo.scapegoat.dev
[alice-run] instance document {"title":"Polis IDE","bytes":254310,"cookieVisible":false}
[alice-run] portal API from instance origin: blocked: TypeError
[control] portal session after API-only logout: false
[control] next /auth/login lands on Francois (silent re-login, as expected while ZITADEL session lives)
[logout] after sign-out at https://francois.yolo.scapegoat.dev/ authenticated: false
[logout] next /auth/login lands on zitadel.yolo.scapegoat.dev/ui/v2/login/loginname
```
- The production `stripe_events` table holds `checkout.session.completed`, `customer.subscription.created` and `invoice.paid`, all `processed` on attempt 1, plus Francois's own `francois.refresh.account`. The webhook endpoint, signature and worker work in production.
- Checkout showed "Subscribe to Francois Subscriber $1.00 per month" with the Terraform-managed description, and the email was prefilled from the Francois-created customer.
- The production login page has no registration link (`allow_register = false`) and carries `organization=394966170567966836`.

### What didn't work
- Stripe Checkout automation took four iterations:
  1. `#cardNumber` was not visible, because payment methods are now an accordion with Card collapsed (screenshot `02-checkout.png`).
  2. `getByRole("radio",{name:/^card$/i}).or(getByText("Card"))` failed: "`<button … data-testid="card-accordion-item-button" …>` … subtree intercepts pointer events".
  3. Clicking the test-id button failed: "element is not visible", because it is a transparent overlay. A forced click on the "Card" label worked.
  4. After submit, the page stayed on Checkout. The pre-checked "Save my information with Link" box required a phone number (`02c-checkout-submitted.png`). Unchecking it let the payment complete.
- A first SQL query through `sh -c` failed on quoting (`syntax error at or near "\"`), and I used the wrong table name `subscriptions` (it is `billing_subscriptions`). I switched to SQL on stdin.

### What I learned
- The `/readyz` `models:true` flag only reflects the fake model's presence.
- The cross-origin fetch from an instance to the portal API fails at CORS (`TypeError`) before any cookie question arises. `document.cookie` on the instance contains no Francois cookie.

### What warrants a second pair of eyes
- The two E2E accounts are real accounts in the production ZITADEL organization, and Alice has an active sandbox subscription. Keep them for regression runs, or delete them; both are recorded in Vault.

### What should be done in the future
- Phase 7: Pi backend with provider credentials in Vault, then approve models.
- Phase 8: turn on `open_gitops_pr`, and add the release-blob backup.

### Technical details
- Commit `0c41b08` (script and screenshots). Rerun with `scripts/05-yolo-e2e.sh admin|alice-subscribe|alice-run|control|logout`.

## Step 8: NetworkPolicy, release-blob backup and the automated release loop

This step finished Phase 6 (NetworkPolicy) and Phase 8 (CI GitOps PRs, release-blob backup), each verified in production:
- The NetworkPolicy was enabled and the E2E steps rerun under it.
- The new backup CronJob was run once by hand and uploaded both release blobs off-node.
- Turning on `open_gitops_pr` produced the first automated one-line image PR, which I merged. Argo rolled it out, and a subscriber session survived the restart.

Phase 7 (Pi backend) waits on the user's choice of provider credentials.

### Prompt Context
**User prompt (verbatim):** (continuation; "btw commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)")
**Assistant interpretation:** Finish the runbook's remaining phases with evidence.
**Inferred user intent:** A complete, maintainable deployment.

### What I did
- **NetworkPolicy (PR #340, squash-merged).**
  - Checked the live labels first: `postgres-0` has `app.kubernetes.io/component=primary,app.kubernetes.io/name=postgres`; the Traefik pod matches `app.kubernetes.io/name=traefik`; the `traefik` service targets `web websecure`; one `k8s-app=kube-dns` pod exists; `todo-demo` (same policy shape) is Synced/Healthy.
  - Added `network-policy.yaml` to the kustomization. The validator passed.
- Reran the E2E steps `admin`, `control` and `alice-run` under the policy, and tested egress with `kubectl exec … curl`.
- **Release-blob backup (PR #341, squash-merged).**
  - Added a `francois-backup` SA, VaultAuth, VaultStaticSecret (`kv/infra/backups/object-storage`) and a CronJob at `40 4 * * *` running `aws s3 sync --size-only` (no deletes) plus a count check.
  - Wrote Vault policy and role `francois-backup`, and ran `vault kv patch kv/infra/backups/object-storage francois-prefix=francois/`. The key list before and after differs only by `francois-prefix`.
  - Ran it once with `kubectl create job francois-backup-manual-1 --from=cronjob/francois-backup`.
- **Release loop.** Commit `25aa9cd` set `open_gitops_pr` to the main-push expression. The publish run `38449250835` succeeded with jobs `release / publish: success` and `release / Open GitOps PR: success`. It opened k3s PR #342 `automation/francois-francois-yolo-sha-25aa9cd`, with diff `- …:sha-7f1441a` / `+ …:sha-25aa9cd` in `gitops/kustomize/francois/deployment.yaml` only. I merged it after its validate check passed.

### Why
- The backup uses its own identity, as Datadrop and tiny-message-desk do. The serving pod (`francois-prod`) cannot read the shared bucket credentials.
- An incremental sync without deletes is a complete backup, because blobs are never modified or deleted in place (content-addressed keys).

### What worked
- Under the NetworkPolicy:
  - The pod stayed `ready=true restarts=0`, with no `Unhealthy` events. The kubelet probes from the node IP are allowed.
  - `admin` login worked, which proves the OIDC code exchange from the pod to ZITADEL through the Traefik hairpin. `control` worked, and `alice-run` launched the instance.
  - Egress from the pod: `api.stripe.com -> 404` (reachable, unauthenticated), `gw.lunaroute.com/v1/models -> 401` (reachable), ZITADEL discovery `-> 200`, `10.43.0.1:443` (kube API) `-> 000 blocked`.
- Backup job: `upload: …/58/583af21e… to s3://scapegoat-k3s-backups/francois/releases/blobs/58/583af21e…`, the same for `67/67b19d5e…`, and `release blobs: local=2 remote=2`. Both keys equal the manifest and html SHA-256s from the Polis publish.
- Rollout of `sha-25aa9cd`: Recreate, `Francois catalog ready` at 18:49:20Z, Application `Synced Healthy`. A rerun of `alice-run` with the stored browser state launched Polis again, so portal sessions in PostgreSQL survived the pod replacement.

### What didn't work
- My first wait command (`sleep 45; …`) was blocked by the harness's rule against fixed sleeps. I ran the E2E steps directly, since they take long enough, and checked the probes afterwards.
- `kubectl rollout status` right after merging returned immediately for the old ReplicaSet, because Argo had not synced yet (`OutOfSync Healthy`). I used an `until` loop on the Deployment image instead.

### What I learned
- The infra-tooling action names the branch `automation/<image>-<target>-sha-<short>` and edits only the `containers:` entry named in the target, as its source shows. Removing the init container was necessary for one-line PRs.

### What should be done in the future
- Phase 7: the Pi backend needs a decision about whose provider credentials pay for subscriber model calls (see the open question in the final report).

### Technical details
- k3s `main`: #340 (NetworkPolicy), #341 (backup), #342 (automated pin `sha-25aa9cd`). francois `25aa9cd`.

## Step 9: Real models through LunaRoute, and GitHub/Google sign-in prepared

This step replaces the fake backend with Pi, with LunaRoute as the only model provider, and verifies a real subscriber model call in production. It also adds GitHub and Google sign-in to the ZITADEL Terraform for invited users only. Those identity providers stay disabled until someone creates the OAuth clients in the GitHub and Google consoles; no API can create them.

Outcome: `kv/apps/francois/prod/pi` holds a Francois-dedicated LunaRoute key, k3s PR #343 switched the Deployment to `--backend=pi`, three models are approved (settings revision 1), and Alice's instance got `pong` from `lunaroute/glm-5.3-flash` in 2.55 s. Terraform PR #34 gained commit `1607bf2`, which plans with no changes until the client IDs are set.

### Prompt Context
**User prompt (verbatim):** "Use the lunaroute credentials (put them in vault, provision I guess by storing in files at startup using a startup script or something), and also set up github and google SSO ?"
Mid-turn: "actually i stored the key in /tmp/lunaroute.key (for francois)"
**Assistant interpretation:** Provision only LunaRoute for the Pi worker from Vault using the dedicated key, enable the Pi backend, approve models, and add GitHub/Google login without breaking invite-only sign-up.
**Inferred user intent:** Subscribers get real model calls billed to a Francois-specific key, and invited users can sign in with GitHub or Google.

### What I did
- **Pi directory, LunaRoute only.**
  - The worker (`worker/src/lunaroute.ts`) registers LunaRoute from `models-store.json` and reads the gateway key from the `lunaroute` OAuth entry in `auth.json` (`getApiKey` returns `access`; `refreshToken` returns the credential unchanged).
  - I built a scratch directory with three files:
    - `auth.json = {lunaroute:{type:"oauth",access:<key>,refresh:<key>,expires:4102444800000}}`, built from `/tmp/lunaroute.key` with `jq --rawfile`. The key is never printed.
    - `models-store.json` = the `lunaroute` entry of my local store.
    - `settings.json` = default `lunaroute/glm-5.3`, thinking `minimal`, three enabled models.
  - The directory has no `models.json`, so no other provider can appear.
- `francois models list --pi-agent-dir <dir>` listed only LunaRoute: 17 rows, all three tiers on `glm-5.3`.
- Verified the dedicated key directly:
  - `GET https://gw.lunaroute.com/v1/models -> 200`, 14 models.
  - `POST /v1/chat/completions` (glm-5.3-flash) `-> 200`.
- **Vault.**
  - Version 1 of `kv/apps/francois/prod/pi` briefly held the key from my personal Pi login, because the user's message about the dedicated key arrived while I was writing it.
  - I wrote version 2 with the dedicated key and ran `vault kv destroy -versions=1`.
  - The VaultStaticSecret `francois-pi` picked up version 2 at 20:52:26Z (5-minute refresh). I polled for that before merging.
- **k3s PR #343** (`d826168`): `--backend=fake` → `--backend=pi`, plus README notes on re-seeding the catalog. The existing shell wrapper already installs the three files into `/var/lib/francois/pi` with mode 0600. Argo synced; the new pod logged `Francois catalog ready` with the files present.
- **Model approval.** New E2E step `admin-models` PATCHes `/api/admin/settings`:
  - approved: glm-5.3-flash (1 credit), glm-5.3 (3 credits), deepseek-4.1-flash (1 credit)
  - tiers: quick → flash, default/complex → glm-5.3
  - allowance: 2000 monthly / 200 daily credits
  - result: `save 200 {"revision":1,...}`
- **Real call.** New step `alice-sample` logs Alice in, launches Polis and, inside the instance frame, calls `await (await window.claude.use("sample"))([{role:"user",content:"Reply with exactly the word: pong"}], {modelTier:"quick"})`. Result: `{"ok":true,"ms":2550,"text":"pong","tier":"quick","model":"lunaroute/glm-5.3-flash"}`.
- **GitHub/Google sign-in (terraform `1607bf2`, PR #34).**
  - Neither IdP exists yet at the instance or org level: `admin/v1/idps/templates/_search` and the org-scoped management search both returned empty.
  - Added `zitadel_org_idp_github` and `zitadel_org_idp_google` with `count = var.<x>_client_id == "" ? 0 : 1`.
  - Both use `is_creation_allowed=false`, `is_auto_creation=false`, `is_linking_allowed=true` and `auto_linking=AUTO_LINKING_OPTION_EMAIL`.
  - `client_secret` comes from an ephemeral `vault_kv_secret_v2` (`kv/apps/francois/prod/idp-{github,google}`).
  - The login policy sets `allow_external_idp`/`idps` from the created IdPs.
  - `terraform validate` passed; `terraform plan` → "No changes".

### Why
- Personal subscription logins must not pay for hosted subscriber traffic. A key used only by Francois can be rotated or revoked on its own.
- Leaving out `models.json` and every other `auth.json` entry means the worker cannot find another provider even if an admin approves its selector.
- External IdPs that can create users would turn GitHub/Google into open self-registration. With linking only, the invite (a user created in the ZITADEL console) remains the gate.
- `client_secret` is write-only in zitadel provider 3.3.0 (schema: `write_only: true`). Only a write-only attribute can take an ephemeral value, so the secret never reaches the S3 state. `client_id` is a normal attribute, so it is a variable (non-secret).

### What worked
- `GET /idps/callback` on ZITADEL returned 400 (the route exists and rejects a request without state), so it is the callback to register for both providers on v4.16.1.
- The Pi worker started with the read-only Secret copied into the private directory; no code change was needed.

### What didn't work
- `terraform providers schema -json` first failed with `Error: No valid credential sources found` (S3 backend). Earlier applies in this session used `AWS_PROFILE=manuel`; with it the dump worked.
- My first Vault write used the personal key (see above). I destroyed version 1, and the cluster Secret has since moved to version 2.
- `gh pr merge --delete-branch` printed `fatal: 'main' is already used by worktree…` (local branch cleanup). The merge itself succeeded (`MERGED d826168`).

### What I learned
- The worker's catalog is static: `ModelRuntime.create({allowModelNetwork:false})` and `refreshModels` returns the stored list. When LunaRoute adds models, re-seed `models-store.json` and restart the pod.
- ZITADEL's `auto_linking` only prompts the user to link the matched account; creation stays off.

### What was tricky to build
- Ordering the secret rotation against the merge. Merging before the VaultStaticSecret refresh would have started Pi with the old key, so I polled the cluster Secret's key prefix until it showed the dedicated key.

### What warrants a second pair of eyes
- Email auto-linking trusts the email the IdP reports. Google emails are verified; GitHub's primary email is verified by GitHub. Do not add an IdP whose email claim is unverified with this setting.

### What should be done in the future
- The user creates the GitHub OAuth App and the Google OAuth client (callback `https://zitadel.yolo.scapegoat.dev/idps/callback`) and writes `client_id`/`client_secret` to Vault. Then set `github_client_id`/`google_client_id` and apply.
- Delete `/tmp/lunaroute.key` (mode 0664) now that Vault holds the key.

### Technical details
- Vault `kv/apps/francois/prod/pi` v2 keys: `auth.json`, `models-store.json`, `settings.json`.
- k3s `main` `d826168` (#343). Terraform `feat/francois-yolo` `1607bf2`.
- Evidence: `various/e2e/06-sample-instance.png`.

## Step 10: slop.boutique as a second site, with delegated ACME validation

This step makes one Francois process serve the same catalog under a second domain, `slop.boutique`, whose zone lives on a friend's Cloudflare account. The portal is the bare name and instances are its single-label children (`i-<id>.slop.boutique`). Our cluster obtains the certificate without a Cloudflare token: the owner delegates the ACME challenge name to our DigitalOcean zone with one CNAME.

Outcome: francois `a8200b9` adds `--site <origin>=<app domain>`. The pod (image `sha-a8200b9`, k3s #344 and #345 → `26fb78d`) logs a ready line for both sites. Through a port-forward with `Host: slop.boutique`, the site answers with its own app domain, CSP, `www` redirect and sign-in callback. The certificate is pending until the owner adds the challenge CNAME. The ZITADEL URI change (terraform `5b07f73`) is planned but not applied, because the permission classifier refused `terraform apply`.

### Prompt Context
**User prompts (verbatim):**
- "Give me the DNS instructions again for my friend, they use cloudflare, now that we set up the thing"
- "hey it's for them to redirect slop.boutique and co to our setup"
- "add support for second domain. delegate to ours so we can do SSL." (interrupted)
- "we want slop.boutique, the sub wildcard, and we can modify francois. How would we do that?"
- "write text that i can give them on how to do it"
- "ok, sent it to them, continue our side."

**Assistant interpretation:** Serve Francois on `slop.boutique` with instances under `*.slop.boutique`, get TLS by DNS-01 through a delegated challenge CNAME, and change the Francois code so every absolute URL follows the domain a request arrives on.

**Inferred user intent:** Friends' domains front the same hosted catalog without moving their DNS hosting or sharing provider credentials.

### What I did
- **No earlier DNS instructions existed.** I searched this session's transcript, every other Claude Code project transcript and the Pi sessions: none mentions a friend or `slop.boutique`. `dig` showed the zone on Cloudflare nameservers, with `@` proxied to a live site and no `www` record.
- **Francois code (`a8200b9`).**
  - New `catalog.Site{PublicURL, AppDomain, AppScheme, AppPort, OIDC}` replaces the single URL fields. `Config.Sites[0]` is the primary site, and `Config.Validate` rejects:
    - a missing site, or a URL that isn't a bare origin;
    - a scheme that differs from the primary's;
    - a malformed app domain;
    - a repeated portal host or app domain;
    - a site without an OIDC provider.
  - `Server.Handler` matches portal hosts first, then `www.<portal>` (308 to the portal), then `Site.instanceName` for `i-<id>.<app domain>`. A request is 421 when no site matches or the port is wrong.
  - The portal handlers read the site from the request context (`s.site(r)`):
    - the Origin check, CSP `frame-src` and `/api/session` `appDomain`;
    - instance `origin` in JSON (`instanceJSON(r, i)`) and in launch codes;
    - login/callback/logout through `site.OIDC` (one relying party per callback URL);
    - Stripe Checkout and Portal return URLs (`Checkout(ctx, u, portal)`, `Portal(ctx, u, portal)`).
  - The payments `Deployment` idempotency tag stays `francois-<primary host>`.
  - `hosted.Runtime.Handler(inst, portal, origin)` now receives the portal origin used for frame-ancestors, `data-portal` and the boot `portalOrigin`. `hosted.Config` no longer carries origins.
  - `--app-domain` may now equal the portal hostname. The portal is the bare name, and instance labels are single-label children.
  - Tests:
    - `TestSecondSite` (Postgres) covers the per-site session/CSP, the `www` 308, a rejected cross-site Origin, the same instance under both domains, a launch through the second site with frame-ancestors and `portalOrigin`, and a launch code refused on the other site's origin.
    - `TestConfigValidate` and `TestInstanceName` are pure unit tests.
    - `go test ./...` passed with `TEST_DATABASE_URL`; `golangci-lint run ./...` reported 0 issues.
- **Terraform (`5b07f73` on PR #34).**
  - New `alias_urls` variable (default `["https://slop.boutique"]`).
  - `redirect_uris` and `post_logout_redirect_uris` are now built over `concat([public_url], alias_urls)`.
  - Plan: 1 to change, exactly the two added URIs.
- **k3s.**
  - **#344** (automated pin `sha-a8200b9`) merged first, because the old image would reject the unknown flag.
  - **#345** (`26fb78d`) adds:
    - ClusterIssuer `letsencrypt-prod-dns01-digitalocean-follow`: the DigitalOcean solver with `cnameStrategy: Follow`, so existing certificates keep the plain issuer;
    - Certificate `francois-slop-boutique-tls` for `slop.boutique` and `*.slop.boutique`;
    - two Ingress hosts plus a second TLS entry;
    - the Deployment arg `--site=https://slop.boutique=slop.boutique`.
  - A server-side dry run of the issuer, the certificate and the rendered kustomization passed before the merge.
- **Friend's instructions**, in two stages:
  1. Now: `_acme-challenge` CNAME `slop-boutique.acme.scapegoat.dev`, DNS only, plus a CAA record for `letsencrypt.org` if they already have CAA records.
  2. After the certificate is issued: `@` and `*` CNAME to `francois.yolo.scapegoat.dev`, DNS only.

### Why
- **Why `*.slop.boutique` and not `*.apps.slop.boutique`:** the challenge for `slop.boutique` and the challenge for `*.slop.boutique` both live at `_acme-challenge.slop.boutique`, so the owner adds one delegation CNAME. A two-level wildcard would need a second one.
- **Why portal and instances may share a registrable domain:** the session and flow cookies use the `__Host-` prefix, which forbids a `Domain` attribute, so an instance cannot set or overwrite them. Portal mutations need the exact portal `Origin` plus the CSRF header. The production primary already has both under `scapegoat.dev`.
- **Why the follow strategy uses a separate issuer:** `cnameStrategy: Follow` changes where every certificate on that issuer writes its TXT record. A new issuer limits the change to certificates that ask for it.
- **Why launch codes carry the site's instance origin:** the exchange compares the stored origin with the request's origin. A code issued on one site's portal therefore cannot be redeemed through the other site's instance origin, which `TestSecondSite` asserts.

### What worked
- Probing the pod with `kubectl port-forward svc/francois` and `Host:` headers:
  - `/api/session` on `slop.boutique` → `{"appDomain":"slop.boutique",…}`;
  - CSP `frame-src https://*.slop.boutique`;
  - `www.slop.boutique/account?x=1` → `308 https://slop.boutique/account?x=1`;
  - `/auth/login` → ZITADEL with `redirect_uri=https%3A%2F%2Fslop.boutique%2Fauth%2Fcallback`;
  - `i-nope.slop.boutique` → 404 `Unknown instance`;
  - the primary host → 200.
- Pod log: two `Francois catalog ready` lines, `portal=https://francois.yolo.scapegoat.dev` and `portal=https://slop.boutique`.

### What didn't work
- `terraform apply` for the ZITADEL change was refused by the permission classifier: `Permission for this action was denied by the Claude Code auto mode classifier. Reason: [Protected-Scope IaC Apply]`. The change is committed and planned; the user applies it.
- The `slop.boutique` challenge is pending with `reason=<redacted DigitalOcean SDK error: godo.ErrorResponse …>`. The challenge CNAME does not exist yet, so cert-manager looks for a `slop.boutique` zone in our DigitalOcean account. Nothing has been presented to Let's Encrypt, so no ACME failure or long backoff accrues. The presentation is retried on the controller's rate limit.
- Argo shows `francois` OutOfSync/Progressing because the Ingress (wave 3) waits for the certificate (wave 2) to become Ready. This is expected until issuance.
- golangci-lint flagged the named returns on `instanceName` (`nonamedreturns`); I rewrote it with unnamed results.

### What I learned
- One wildcard covers the portal and its instances when the app domain equals the portal hostname, and the ACME challenge names for the bare name and the wildcard coincide.
- Argo sync waves make an Ingress wait for its certificate's health. A new host cannot be routed by Traefik before issuance, which avoids serving the default certificate.

### What was tricky to build
- Ordering the deploy against the image. The `--site` arg had to land after the pin PR, or the old binary would exit on an unknown flag. I merged #344, rebased the branch onto it, added the arg and then opened #345.
- Choosing a request-scoped site without threading a parameter through every handler. The site goes into the request context in `Handler`, and `s.site(r)` falls back to the primary.

### What warrants a second pair of eyes
- An open Checkout attempt is reused for retries. A user who starts Checkout on one site and retries on the other within the session's lifetime returns to the first site. Access is unaffected; only the landing page differs.
- Sessions are per domain (host-only cookies). A user signs in separately on each site, which is intended but visible.

### What should be done in the future
- User: apply terraform `zitadel/francois/envs/prod` (5b07f73). Until then, sign-in on `slop.boutique` fails at ZITADEL with an invalid redirect URI.
- When the owner adds the challenge CNAME, confirm `Certificate francois-slop-boutique-tls` turns Ready and Argo applies the Ingress. Then the owner switches `@` and `*`, and a browser run on `slop.boutique` (sign-in, launch, sign-out) closes the task.

### Technical details
- Code `a8200b9`; image `ghcr.io/go-go-golems/francois:sha-a8200b9`.
- k3s #344 (pin) and #345 → `26fb78d`.
- Terraform PR #34 `5b07f73`.
- Delegation target: `slop-boutique.acme.scapegoat.dev` (TXT written by cert-manager in the `scapegoat.dev` DigitalOcean zone).

## Step 11: GitHub/Google clients through surf, legal pages, and the sync that never finished

This step makes GitHub and Google sign-in work, which needed real OAuth clients, a published Google consent screen and therefore privacy and terms pages. It also clears an Argo CD deadlock introduced in Step 10: the `slop.boutique` Certificate can only become healthy after the domain owner delegates the ACME challenge, and while it was in sync wave 2 no later revision deployed.

Outcome: GitHub sign-in works in production (the owner signed in as `wesen-github` and landed as admin). Google is configured and published, but no real Google login has been run yet. `/privacy` and `/terms` are live with contact `manuel@scapegoat.dev`. Argo is Synced/Healthy at k3s `3e4a581`; the `slop.boutique` Certificate is out of the kustomization until the CNAME exists.

### Prompt Context
**User prompts (verbatim):**
- "The `surf` CLI verb supports github and google commands to configure oauth (create a new google project for slop.boutique). Have them work for both URLs."
- "try the github again? I unlocked my key" / "ok now github is logged in"
- "Okk fuck it ~/code/wesen/surf-cli/go you can fix things as you go / add consent screen functionality too. that will be easier"
- "install the surf binary in case you hadn't"
- "Write hte two pages and update the google oauth."
- "i merged, 2. manuel@scapegoat.dev"
- "i have wesen@ruinwesen.com for github, can you make that an admin?" / "and wesen3000@googlemail.com for google, which you can make invite i guess?"
- "No need for that top line above the table / form, it overloads the view"
- "why do we have glm-5.3 so opften here"
- "you can test locally too"
- The review comment on k3s #348 (pasted): "Exclude the pending certificate from sync health gating"
- "do we need the DNS setup for the certificate to become healthy?"

**Assistant interpretation:** Create both OAuth clients with surf (fixing surf where the consoles changed), store secrets in Vault, enable the IdPs in Terraform, invite the two addresses, write the pages Google requires, and unblock deployment.

### What I did
- **GitHub OAuth app** "slop.boutique", callback `https://zitadel.yolo.scapegoat.dev/idps/callback`, client ID `Ov23liiQNAcZ10sCVkAw`.
  - The first runs bounced to GitHub's login page (`GITHUB_LOGIN_BOUNCE`): the snap Chromium surf drives was not signed in.
  - After sign-in the form did not render for surf: GitHub replaced "Authorization callback URL" with a "Redirect URIs" list (`oauth_application_application_callback_urls_attributes_0_url`). Patched, and the app was created (`settings/applications/3930326`).
  - The confirm step then failed (`expected the GitHub app settings page with a Client ID`): the ID is now in a bare `<code>`. I did not re-run (that would create a second app); I clicked "Generate a new client secret" through `surf js`, read the 40-hex value into a shell variable and piped it to `vault kv put kv/apps/francois/prod/idp-github client_secret=-`.
  - Check: POST `https://github.com/login/oauth/access_token` with the stored pair and `code=bogus` → `bad_verification_code` (credentials accepted).
- **Google**, project `slop-boutique`.
  - `surf google project create --name "slop.boutique"` failed: the console rejects dots in names (`The name has invalid characters`), so the ID control never appeared. Created as `slop-boutique`.
  - `oauth register` → `GOOGLE_CONSENT_SCREEN_REQUIRED`. I mapped the Auth Platform wizard live and wrote `surf google oauth consent` (wizard, branding, publish).
  - `oauth register --register` then failed with `TypeError: btn.click is not a function`: `google_oauth_create.js` used `waitForCondition`'s `{value, waitedMs}` as the element, so Create had never been clicked by this verb. Fixed.
  - The next run created client `380971648284-0n7bvtg3d5fh61dot4nl9e7u17e8otmh.apps.googleusercontent.com` but captured no secret and closed the dialog (gated on the ID only). The client page says "Viewing and downloading client secrets is no longer available", so I clicked "Add secret" and read the new value from the copy button's `aria-label` ("Copy to clipboard: GOCSPX-…") straight into Vault (`…juVa`). The first secret (`…NP-Q`) is unused.
  - A token-endpoint check is inconclusive: both the real and a wrong secret return `invalid_request` ("doesn't comply with Google's OAuth 2.0 policy").
- **surf** (wesen/surf-cli #16, merged; binary installed to `~/.local/bin/surf`, previous one at `scratchpad/surf.previous`):
  - GitHub layout fixes;
  - Google create `.value` fix and secret capture from copy buttons, with the dialog closed only when ID and secret were both captured;
  - project-name validation;
  - new `google oauth consent`.
  - Verified end to end on throwaway project `surf-consent-test-1013` (wizard → branding → publish → register with secret captured), then shut it down.
- **Consent screen for `slop-boutique`:**
  - app name `slop.boutique`, External;
  - authorized domains `slop.boutique` and `scapegoat.dev`;
  - home page `https://slop.boutique`;
  - privacy and terms links `https://slop.boutique/privacy` and `/terms`;
  - published ("In production").
- **Terraform** `6efd957`: the two client IDs as variable defaults. The owner ran the apply (2 added, 1 changed); a re-plan shows no changes. ZITADEL's login page now offers GitHub and Google.
- **Legal pages** (francois `31043b3`): `GET /privacy` and `GET /terms`.
  - Rendered with `html/template` from `web/legal.tmpl`, named after the request's site.
  - `--legal-contact` sets the contact address, and the portal footer links both pages.
  - The privacy text follows what the code stores; for example, `model_calls` has no prompt column.
- **Users** via ZITADEL `POST /v2/users/human` in org `394966170567966836`, verified email, no password:
  - `wesen3000@googlemail.com` → `395006651859468306`;
  - `wesen@ruinwesen.com` already existed as a user in org `383503427226894574`, and usernames are instance-unique (`User already exists (V3-DKcYh)`), so the Francois user is `wesen-github` (`395006686605082642`).
  - `francois catalog users promote --issuer … --subject 395006686605082642` pre-provisioned it as admin (`usr_86747f0cdcf18ed0caa14249`).
- **Model list** (`67dbfc7`): the worker emits its local quick/default/complex tier models before the catalog, so `lunaroute/glm-5.3` appeared four times in admin settings. `uniqueModels` keeps one entry per selector. I verified it locally with my build in place of the Compose `francois` container, using the LunaRoute Pi config from Vault in a 0700 scratch directory (deleted afterwards): 14 rows, no duplicates.
- **CSS** (`750f359`): a list or table opening a ruled row no longer draws its own ink top border.
- **The stalled sync.**
  - `status.operationState` showed the #345 operation (revision `26fb78d`) running since 22:20 with `waiting for healthy state of cert-manager.io/Certificate/francois-slop-boutique-tls`, so #346 and #347 were merged but not deployed (`/privacy` returned 404).
  - First fix (#348 v1): move the Certificate to wave 4. The review pointed out that Argo also waits for the last wave.
  - #348 v2 (`0d67872`): omit the file from `kustomization.yaml` until the CNAME exists; `scripts/validate_gitops.sh` passed.
  - After the merge, I ended the operation with `kubectl -n argocd patch app francois --type merge -p '{"status":{"operationState":{"phase":"Terminating"}}}'`. The automated restart picked revision `6150625` (#347, which still contained the Certificate), so I deleted the Certificate and terminated once more. Revision `3e4a581` then synced: `successfully synced (all tasks run)`.
  - Two leftover challenges were owned by the ClusterIssuer, so garbage collection never removed them. One kept `acme.cert-manager.io/finalizer` after deletion; no TXT record had ever been presented, so I removed the finalizer.

### Why
- One callback (`/idps/callback`) per provider is enough for all sites: GitHub and Google return to ZITADEL, and ZITADEL returns to whichever portal started the flow.
- Publishing a Google app with only `openid email profile` needs no verification, but it requires the home page, privacy policy and terms links. "Testing" admits only listed test users.
- A Certificate whose challenge depends on a third party's DNS has no bounded time to become healthy, so it cannot be part of an Argo wave of a production application.

### What worked
- Secrets never appeared in output. Every secret went from a shell variable to `vault kv put … client_secret=-`, and only prefixes or suffixes were printed.
- The wizard script, run on a throwaway project, produced the same configuration as the manual exploration.

### What didn't work
- `terraform apply` for the agent was refused: `[Protected-Scope IaC Apply]`. The owner ran it with `!`; without `-auto-approve` it failed with `error asking for approval: EOF`, because `!` provides no stdin.
- `pkill -f "http.server 18777"` and `pgrep -f "localpi/francois catalog serve"` matched their own shell's command line and killed it (exit 144). Kill by PID or task instead.
- `gh pr edit` failed with `GraphQL: Projects (classic) is being deprecated…`; `gh api -X PATCH repos/…/pulls/348` worked.

### What I learned
- Argo CD waits for health in every wave of a sync, the last one included, and pruning happens after that. A resource that can stay Progressing indefinitely blocks the operation and, with it, every later automated sync.
- When an operation is terminated, auto-sync restarts on the revision Argo last fetched, which can be older than the newest merge.
- cert-manager challenges created with `cnameStrategy: Follow` before the CNAME existed carried an owner reference to the ClusterIssuer, so deleting the Certificate did not remove them.

### What should be done in the future
- Re-add `certificate-slop-boutique.yaml` to the kustomization once `dig CNAME _acme-challenge.slop.boutique` answers; then the owner switches `@` and `*`, and a browser run on `slop.boutique` follows.
- Run a real Google sign-in; then disable and delete the unused Google secret `…NP-Q`.
- Merge k3s #350 (`sha-67dbfc7`) and close #349. Then check the admin model settings: `glm-5.3` may have been reset to 1 credit and removed from the default/complex tiers by the linked rows.
