import { build } from "esbuild";
import { mkdir } from "node:fs/promises";
await mkdir("worker/dist", { recursive: true });
await build({
  entryPoints: ["ui/src/app.tsx"],
  bundle: true,
  outfile: "pkg/web/static/app.js",
  format: "iife",
  minify: true,
  jsx: "automatic",
  legalComments: "eof",
});
await build({
  entryPoints: ["ui/src/runtime.ts"],
  bundle: true,
  outfile: "pkg/web/static/runtime.js",
  format: "iife",
  minify: true,
  legalComments: "eof",
});
await build({
  entryPoints: ["worker/src/main.ts"],
  bundle: true,
  outfile: "worker/dist/main.js",
  format: "esm",
  platform: "node",
  packages: "external",
  legalComments: "eof",
});
