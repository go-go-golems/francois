---
Title: Deploy the hosted Francois catalog to the scapegoat yolo K3s cluster with Argo CD
Ticket: FRANCOIS-003
Status: active
Topics:
    - deployment
    - kubernetes
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Design and phased runbook for running the hosted Francois catalog on the yolo K3s cluster via Argo CD; drafts validated, nothing applied yet."
LastUpdated: 2026-10-13T12:26:52.381572884-04:00
WhatFor: ""
WhenToUse: ""
---

# Deploy the hosted Francois catalog to the scapegoat yolo K3s cluster with Argo CD

## Overview

Plan for deploying `francois catalog serve` to the single-node Hetzner K3s cluster (`*.yolo.scapegoat.dev`): portal at `francois.yolo.scapegoat.dev`, instance origins at `i-<id>.apps.francois.yolo.scapegoat.dev` (DNS-01 wildcard certificate), production ZITADEL, shared PostgreSQL 16, Vault/VSO secrets mounted as files, Stripe sandbox, one-replica Recreate Deployment, CI image pins through infra-tooling.

- Guide: [design-doc/01-yolo-k3s-deployment-analysis-design-and-intern-implementation-guide.md](./design-doc/01-yolo-k3s-deployment-analysis-design-and-intern-implementation-guide.md)
- Diary: [working-note/01-deployment-investigation-diary.md](./working-note/01-deployment-investigation-diary.md)
- Draft GitOps package, Vault policies/roles, bootstrap script: `reference/gitops-draft/`
- Draft ZITADEL Terraform environment: `reference/terraform-draft/zitadel/francois/envs/prod/`
- Measurement and validation scripts: `scripts/01-…04-…`

Status: design delivered; the deployment itself (Phases 0–8 in tasks.md) has not started.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- deployment
- kubernetes
- backend

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
