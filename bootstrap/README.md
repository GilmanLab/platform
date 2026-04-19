# Bootstrap Artifacts

This subtree is the canonical source for platform-owned bootstrap/core operator
delivery.

Each operator directory is an Argo-consumable directory source:

- `chart.yaml` is the child Argo CD `Application` that installs the pinned
  upstream chart using values from this repo
- sibling plain manifests are applied from the same directory
- `render/` contains the raw Talos bootstrap artifacts
- `values/` contains the canonical operator values inputs

The tracked render surface is:

- `cilium/render/bootstrap.yaml`
- `argocd/render/bootstrap.yaml`
- `argocd/render/full.yaml`
- `kro/render/full.yaml`

`cilium` also has a local-only `full` render under `.state/render/` because the
full install shape may render secret-bearing content.

Use:

- `just render` to refresh tracked artifacts
- `just render-all` to also materialize local-only outputs
- `just validate` to check tracked artifacts are current and do not embed secret
  material
