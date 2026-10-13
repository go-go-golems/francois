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
