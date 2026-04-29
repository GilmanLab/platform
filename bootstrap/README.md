# Bootstrap Artifacts

This subtree is the canonical source for platform-owned bootstrap/core operator
delivery.

Most operator directories are wrapper Helm charts:

- `Chart.yaml` defines the released chart version and the pinned upstream chart
  dependency
- `values.yaml` is the steady-state GitOps install shape
- `bootstrap-values.yaml` carries the narrow bootstrap-only overrides when a
  Talos-safe lane differs from the steady-state install
- `templates/` contains platform-owned manifests layered on top of the upstream
  chart
- `render/` contains the tracked raw manifests consumed by Talos day-0 inputs

`bootstrap/k0s` is the exception. It is a released bootstrap image directory,
not a wrapper chart. It packages a temporary VyOS-hosted k0s management plane
and publishes the resulting image to GHCR for `infra` to consume by release tag.

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
- `moon run bootstrap-k0s:check --summary minimal` to lint and build the
  `bootstrap-k0s` image locally
