# GilmanLab Schemas

This module contains reusable schema contracts for GilmanLab platform tooling.

CUE definitions are the source of truth. Generated Go structs are committed so
Go consumers can depend on the same contract without duplicating hand-written
types.

The first package is intentionally narrow:

- `incusos` defines the IncusOS image build and seed contract used by future
  `labctl` image commands.

Validate and regenerate from this directory:

```sh
cue fmt --check --files .
cue mod tidy --check
cue vet -c ./...
bash scripts/generate-go.sh
go test ./...
```
