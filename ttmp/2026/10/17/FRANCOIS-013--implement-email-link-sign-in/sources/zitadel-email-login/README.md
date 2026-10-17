# ZITADEL email-only login sources

Collected 2026-10-17 for FRANCOIS-013. Start with the [source analysis](../../analysis/01-zitadel-email-only-magic-link-feasibility-and-source-trace.md) and [current implementation guide](../../design-doc/01-email-link-sign-in-intern-implementation-guide.md).

- Five raw official documentation/discussion pages, with readable Markdown extracts.
- `manifest.json`: page URL, retrieval time, response size and digest.
- `v4.16.1/`: selected upstream files matching the deployed release.
- `source-manifest.json`: exact upstream revision, source URLs and digests.
- `deployment-version.json`: read-only observed image names; no secret/configuration export.

To refresh page snapshots, run `python3 ../../scripts/02-save-zitadel-sources.py` from this folder. Re-running changes retrieval timestamps and may capture newer documentation. Preserve the version-pinned source when comparing current docs to the deployed API.

The source audit used `git clone --depth 1 --filter=blob:none --sparse https://github.com/zitadel/zitadel.git /tmp/francois-013-zitadel-source`, then fetched tag v4.16.1 and checked it out detached. The saved source revision is cc3812a940d629984802fd1246f84fd91d07f6e8. Selected files are snapshots, not application dependencies or vendored packages. No runtime authentication success is claimed.
