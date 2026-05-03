# GilmanLab Lab Schemas

This module contains shared CUE schemas for GilmanLab lab tooling.

The CUE definitions are the source of truth. Generated Go types are committed
for Go consumers that need typed access to the same contracts.

Packages:

- `incusos` — IncusOS image build and seed configuration.
- `talos` — Talos Image Factory download and NoCloud config artifact builds.

Validate and regenerate from this directory:

```sh
cue fmt --check --files .
cue mod tidy --check
cue vet -c ./...
bash scripts/generate-go.sh
go test ./...
```
