# GilmanLab Lab Schemas

This module contains shared CUE schemas for GilmanLab lab tooling.

The CUE definitions are the source of truth. Generated Go types are committed
for Go consumers that need typed access to the same contracts.

Validate and regenerate from this directory:

```sh
cue fmt --check --files .
cue mod tidy --check
cue vet -c ./...
bash scripts/generate-go.sh
go test ./...
```
