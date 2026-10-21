# slop uploader

A form on your own machine for getting prototypes onto
[slop.boutique](https://slop.boutique): drop in an HTML file, fill out the
details, cover, gallery and write-up, and publish in one click.

## Start it

Needs Node 20 or newer; no packages to install.

```sh
npm start          # opens http://127.0.0.1:4747
```

It reads your personal API token from `~/.config/francois/token` (or
`FRANCOIS_TOKEN`). Create one on https://slop.boutique/account → API tokens
with the `studio` scope. The token stays in the local server; the page in your
browser never sees it.

| Variable | Default | |
| --- | --- | --- |
| `PORT` | `4747` | local port |
| `SLOP_SERVER` | `https://slop.boutique` | catalog to upload to |
| `FRANCOIS_TOKEN_FILE` | `~/.config/francois/token` | token file |
| `NO_OPEN` | unset | don't open the browser |

## Using it

- **New prototype:** leave *Artifact* on "New artifact" and drop the HTML.
  The title comes from the file's `<title>`, the ID from the title.
- **New version:** pick the artifact. The form fills in from the site and
  suggests the next version; drop the new HTML. A file named like an
  existing artifact (`lattice-rift.html`) selects it for you.
- **Check** runs the site's validation and stores nothing.
- **Save as draft** uploads everything but publishes nothing; preview and
  publish it in the studio.
- **Publish** makes the release and the storefront page live.

Media (cover, gallery) uploads first, then the release, its attachments, the
title/description/tags, and the storefront page. If a step fails, the log
says which, and a release that was already created stays as a draft in the
studio.

## Development

```sh
npm test
```

`server.mjs` serves `public/` and forwards `/api/me` and `/api/studio/*` to
the catalog with the token. It answers only requests addressed to
`127.0.0.1`/`localhost` on its port, from its own origin, carrying a secret
embedded in the page, so another website cannot use your token through it.
