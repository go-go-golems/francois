#!/usr/bin/env python3
"""Capture the two current byte-consumer paths without credentials or blobs."""
from pathlib import Path
import subprocess
base=Path(__file__).resolve().parents[1]
repo=Path(__file__).resolve().parents[6]
revision=subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip()
sections=[('pkg/catalog/routes.go','func (s *Server) download(', '\nfunc (s *Server) account('),('pkg/catalog/hosted/routes.go','func (rt *Runtime) page(', '\nvar accepted =')]
out=['# Current download and hosted HTML paths',f'\nInspected repository revision: `{revision}`. Read-only source excerpts; no user data or runtime bootstrap values.\n']
for name,start,end in sections:
 source=(repo/name).read_text();lo=source.index(start);hi=source.index(end,lo)
 out.extend([f'## {name}', '```go',source[lo:hi].rstrip(),'```'])
(base/'sources/current-byte-paths.md').write_text('\n\n'.join(out)+'\n')
print('Saved current download and hosted response source excerpts.')
