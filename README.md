# Francois

A Go host for saved Claude HTML artifacts, using Pi's local configuration for model access.

Initialized from `go-go-golems/go-template` commit
`5bf14bfd4a7b13124dbe9f2c3078c29a33627a11` (local checkout).
Module: `github.com/go-go-golems/francois`. Binary: `francois`.

```sh
go run ./cmd/francois --help
go run ./cmd/francois --version
go test ./...
go build ./...
make logcopter-check
make lint
```

The CLI currently provides the command root, Glazed logging, and help.
The hosting service is designed in docmgr ticket **FRANCOIS-001** under
`ttmp/2026/10/11/FRANCOIS-001--host-claude-artifacts-with-glazed-and-pi-model-runtime/`.
Read its design document for the API contract, Pi integration, and implementation phases.

Template CI, hooks, linting, generation, and release files are included.
The template release workflow still requires its publishing credentials and infrastructure;
no release has been configured or dispatched. There is no Git remote configured here.
