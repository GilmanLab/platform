# Bootstrap Artifacts

This subtree is the canonical source for platform-owned bootstrap/core operator
delivery.

Each operator directory is a wrapper Helm chart:

- `Chart.yaml` defines the released chart version and the pinned upstream chart
  dependency
- `values.yaml` is the steady-state GitOps install shape
- `bootstrap-values.yaml` carries the narrow bootstrap-only overrides when a
  Talos-safe lane differs from the steady-state install
- `templates/` contains platform-owned manifests layered on top of the upstream
  chart
- `render/` contains the tracked raw manifests consumed by Talos day-0 inputs

The tracked render surface is:

- `cilium/render/bootstrap.yaml`
- `argocd/render/bootstrap.yaml`
- `argocd/render/full.yaml`
- `kro/render/full.yaml`

`cilium` also has a local-only `full` render under `.state/render/` because the
full install shape may render secret-bearing content. Helm dependency payloads
are materialized under `charts/` during local render and validation and are
ignored.

Use:

- `just render` to refresh tracked artifacts
- `just render-all` to also materialize local-only outputs
- `just validate` to lint the charts, refresh dependencies, and confirm tracked
  artifacts are current and free of embedded secret material
