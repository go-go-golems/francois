# Tasks

## TODO
- [x] Write the yolo deployment analysis/design/implementation guide with draft manifests <!-- t:m63p -->
- [x] Upload the guide to reMarkable <!-- t:j02g -->
- [x] Phase 0: answer Q1 (image visibility) and Q2 (Stripe sandbox account); confirm hostnames (D1) <!-- t:hcu7 -->
- [x] Phase 1: create GitHub remote; implement C1 --oidc-extra-scopes; add root Dockerfile, deploy/gitops-targets.json, publish-image.yaml <!-- t:ba08 -->
- [x] Phase 2: apply terraform zitadel/francois, DNS *.apps.francois.yolo record, Stripe francois catalog root <!-- t:0c26 -->
- [x] Phase 3: enable sandbox Customer Portal; create webhook endpoint, secret straight to Vault <!-- t:muee -->
- [x] Phase 4: Vault policies/roles, GitHub App credential, bootstrap-francois-secrets.sh <!-- t:kwv4 -->
- [x] Phase 5: copy GitOps package, apply prod-apps project + Application, first sync and smoke tests <!-- t:t2u0 -->
- [x] Phase 6: first admin, first release, subscriber end-to-end in browser; then NetworkPolicy <!-- t:td2j -->
- [ ] Phase 7: Pi backend (Vault pi record, --backend=pi, approve models) <!-- t:59st -->
- [ ] Phase 8: enable CI GitOps PRs (check Q4 init image line); release-blob backup CronJob and restore check <!-- t:r3f0 -->
