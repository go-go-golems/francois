# Changelog

## 2026-10-13

- Initial workspace created

## 2026-10-13

Created ticket; wrote yolo K3s deployment guide (14 sections, 7 decisions, 8 gaps, 9 phases) with validated draft GitOps package, Vault policies/roles, bootstrap script and ZITADEL Terraform env; measured probe Host 421, PostgreSQL 16 compatibility and DNS resolution

## 2026-10-13

Uploaded guide + diary bundle to reMarkable: /ai/2026/10/13/FRANCOIS-003/FRANCOIS-003 Yolo K3s Deployment Guide.pdf (remarquee: OK uploaded)

## 2026-10-13

Implemented sign-out through ZITADEL end_session, --oidc-extra-scopes, invite-only --signup-allowed, root Dockerfile, publish workflow; public repo go-go-golems/francois; image ghcr.io/go-go-golems/francois:sha-72999cc (public); Stripe francois catalog imported+applied; PRs wesen/2026-03-27--hetzner-k3s#339 and wesen/terraform#34 open

## 2026-10-13

Deployed: Argo francois Synced/Healthy, wildcard cert issued, smoke tests pass, Stripe webhook enabled (k3s 0e06f98, image sha-7f1441a)

## 2026-10-13

NetworkPolicy (#340), release-blob backup verified (#341), automated GitOps PR loop verified (#342, sha-25aa9cd live); guide gained implementation-status section

## 2026-10-13

Pi backend live with LunaRoute only (k3s #343); models approved; real subscriber call verified. GitHub/Google invited-user sign-in added to Terraform PR #34 (disabled until OAuth clients exist).

## 2026-10-13

Second site slop.boutique: catalog serve --site (a8200b9), follow-CNAME DNS-01 issuer + certificate + ingress (k3s #345), ZITADEL alias URIs planned (terraform 5b07f73, apply pending)

## 2026-10-13

ZITADEL alias URIs for slop.boutique applied by the user (re-plan: no changes); authorize with redirect_uri=https://slop.boutique/auth/callback reaches the login UI
