# Changelog

## 2026-10-15

- Initial workspace created

## 2026-10-15

Created the ticket and wrote the intern guide: scoped tokens, customer-management endpoints, pkg/catalog/client, the francois catalog command tree and an MCP server (stdio, then remote /mcp), with nine decision records and six phases.

### Related Files

- /home/manuel/code/wesen/go-go-golems/francois/ttmp/2026/10/15/FRANCOIS-007--full-api-client-customer-management-and-an-mcp-server-for-the-hosted-catalog/design-doc/01-full-api-client-customer-management-and-mcp-server-analysis-design-and-implementation-guide.md — The guide

## 2026-10-15

Uploaded the guide to reMarkable at /ai/2026/10/15/FRANCOIS-007.

## 2026-10-15

Recorded owner decisions (subscriber tokens: yes; catalog push → catalog releases push) and a measured go-go-mcp vs go-sdk dependency comparison in the MCP library decision.

## 2026-10-15

MCP library decision changed to go-go-mcp v0.3.0 (embedded OIDC no longer linked; introspection available); §6.7-6.8, phases and tests rewritten for its API.

## 2026-10-15

Phase 1 implemented: scoped tokens (migration 008, guard scopes on every route), /api/me, /api/tokens with minting rules, admin token oversight, token IDs in the audit log, account-page token UI.
