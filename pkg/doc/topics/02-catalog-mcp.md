---
Title: Letting an agent manage the catalog (MCP)
Slug: catalog-mcp
Short: Run francois mcp so Claude Code can manage a hosted catalog with a scoped token.
Topics:
- catalog
- mcp
- agents
Commands:
- mcp
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

`francois mcp` is a Model Context Protocol server. An AI application such as
Claude Code starts it as a subprocess and talks to it over stdio; the server
offers the catalog's API as tools (`find_customers`, `grant_complimentary_access`,
`push_release`, `update_plan`, … about 70 in total). Every tool calls the
hosted catalog's HTTP API with your token, so the site's permission checks
apply and the audit log records the token.

## Setup with Claude Code

Create a token on the site's account page with the scopes the agent should
have, store it in a file only you can read, and register the server:

```bash
umask 077; printf '%s' 'frt_…' > ~/.config/francois/agent-token
claude mcp add francois -- francois mcp \
  --server https://slop.boutique --token-file ~/.config/francois/agent-token \
  --allow-dir ~/code/my-artifacts
```

Prefer `--token-file` over an environment variable in the MCP configuration:
the configuration file is stored in plain text.

## Choosing what the agent may do

- **Scopes** decide which tools exist: a `studio` token gets content tools, an
  `admin:read` token gets read-only customer tools, `admin:write` adds changes,
  `account` gets your own instances.
- `--toolsets studio,customers,account` narrows further.
- `--read-only` offers only tools that change nothing — a good default for an
  agent that answers questions about customers.
- `--list-tools` prints what would be offered.

Tools that remove or end something (disable, revoke, withdraw, delete,
cancel, suspend) are marked destructive, so MCP clients ask before running
them.

## Files

Upload tools (`push_release`, `add_attachment`, `upload_media`) read local
files only under `--allow-dir` (default: the directory the server was started
in), after following symlinks. They also accept inline content
(`html`, `contentBase64` with `filename`).

## Remote MCP (`catalog serve --mcp`)

With `--mcp`, the server itself offers the same tools at `https://<portal>/mcp`
(streamable HTTP, stateless). Agents authenticate with a personal API token in
the `Authorization: Bearer` header; for Claude Code:

```bash
claude mcp add --transport http francois https://slop.boutique/mcp \
  --header "Authorization: Bearer frt_…"
```

Each tool call is replayed through the API in-process with the caller's
token, so permission checks and the audit log behave as for any API call.
Every tool is listed, and the token's scopes are enforced when a tool is
called. Remote tools take uploads inline (`html`, `contentBase64`); local
paths are refused. Claude.ai connectors need OAuth sign-in instead of a
pasted token; that is not supported yet.
