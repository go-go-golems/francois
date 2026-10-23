#!/usr/bin/env bash
# List the window.claude capabilities an artifact uses, to write its release
# manifest. usage: 02-capabilities.sh <file.html | dir-with-js>...
set -euo pipefail
for src in "$@"; do
  if [[ -d $src ]]; then files=$(find "$src" -name '*.js' -o -name '*.html'); else files=$src; fi
  echo "== $src"
  # claude.use("sample"), claude.use?.('room') …
  cat $files | grep -oE "use\??\.?\(\s*[\"'][a-z]+[\"']" | grep -oE "[\"'][a-z]+[\"']" | tr -d "\"'" | sort | uniq -c
  # direct calls such as window.claude.complete / claude.downloads
  cat $files | grep -oE 'claude\.[a-zA-Z]+' | sort | uniq -c | sort -rn | head -10
  # external scripts and import maps that must stay reachable from the sandbox
  cat $files | grep -oE '<script[^>]*src="[^"]*"|https://(cdn|unpkg|esm)[^"'"'"' )]*' | sort -u | head -10
done
