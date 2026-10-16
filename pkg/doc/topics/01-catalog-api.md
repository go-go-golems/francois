---
Title: Managing a hosted catalog from the command line
Slug: catalog-api
Short: Use francois catalog commands with a scoped personal API token.
Topics:
- catalog
- api
- tokens
Commands:
- catalog
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

`francois catalog` manages a hosted catalog such as https://slop.boutique over
its HTTP API. The commands never touch the database: every call passes through
the same authorization checks as the browser, and the audit log records which
token made each change.

## Tokens and scopes

Create a token on the site's account page (`/account` → API tokens). Choose
only the scopes the task needs:

| Scope | Reaches |
| --- | --- |
| `account` | your account, Stripe links, your instances |
| `studio` | artifacts, releases, media, posts, creators you may edit |
| `admin:read` | reading customers, instances, billing, usage, settings, audit |
| `admin:write` | changing them (implies `admin:read`); expires within 90 days |

A scope never exceeds your role. `francois catalog whoami` shows what a token
can do.

## Connecting

```bash
export FRANCOIS_SERVER=https://slop.boutique
export FRANCOIS_TOKEN=frt_…        # or --token-file path
francois catalog whoami
```

## Command groups

- Content (`studio`): `artifacts`, `releases` (including `releases push`),
  `attachments`, `media`, `posts`, `creators`.
- Customers (`admin:*`): `users`, `subscriptions`, `instances`, `plans`,
  `settings`, `billing`, `usage`, `audit`.
- Yourself (`account`): `account`.
- Operators inside the deployment: `db` writes to the database directly, for
  bootstrap (promoting the first administrator) and break-glass use.

Every command prints rows through Glazed: `--format json|yaml|csv|table` and
`--output-fields a,b`. Commands that change or remove data ask for confirmation
on a terminal and need `--yes` in scripts.

## Examples

```bash
francois catalog users list --role subscriber --plan studio
francois catalog users grant usr_… --days 30 --reason "beta tester" --plan studio
francois catalog users revoke-grant agr_… --yes
francois catalog subscriptions cancel sub_… --yes      # at period end; refunds stay in Stripe
francois catalog plans set studio --monthly-credits 1500 --yes
francois catalog releases push --artifact-id polis --version 1.3.0 --html dist/polis.html --publish
francois catalog audit list --actor usr_… --limit 20
```
