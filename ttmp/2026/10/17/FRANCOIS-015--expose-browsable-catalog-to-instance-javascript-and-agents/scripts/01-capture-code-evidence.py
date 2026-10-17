#!/usr/bin/env python3
"""Capture bounded, line-numbered primary source evidence; no credentials."""
from pathlib import Path
import subprocess
# Resolve through git instead of depending on ticket date nesting.
root=Path(subprocess.check_output(['git','rev-parse','--show-toplevel'], text=True).strip())
blocks=[('pkg/catalog/pages.go',406,435),('pkg/catalog/pages.go',579,600),('pkg/catalog/routes.go',244,302),('pkg/catalog/hosted/runtime.go',113,179),('pkg/catalog/hosted/runtime.go',194,215),('pkg/catalog/hosted/rpc.go',46,75),('pkg/catalog/hosted/sample.go',211,264),('ui/src/runtime.ts',790,832),('pkg/piworker/validate.go',65,89),('examples/imports/rag-workbench/source/agent.js',474,487),('pkg/catalog/releases/releases.go',123,148)]
revision=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
out=['# Primary code evidence', '',f'Captured from working tree based on commit `{revision}`. Other concurrent ticket work may be present; excerpts show actual bytes at capture time. No claim of production configuration is made.', '']
for name,start,end in blocks:
 lines=(root/name).read_text().splitlines()
 out.extend([f'## {name}:{start}', '', '```text']+[f'{i}: {line}' for i,line in enumerate(lines[start-1:end],start)]+['```',''])
Path(__file__).resolve().parents[1].joinpath('sources/code-evidence.md').write_text('\n'.join(out))
