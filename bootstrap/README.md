# Bootstrap Artifacts

This subtree is the canonical source for platform-owned bootstrap manifests.

The tracked render surface is intentionally narrow:

- `cilium/render/public.yaml` is the Talos-consumable public-safe artifact
- `argocd/render/install.yaml` is the tracked Argo CD bootstrap artifact
- `kro/render/install.yaml` is the tracked kro bootstrap artifact

`cilium` also has a private render lane for the future full install shape. Its
output is generated into a local `.state/` path and is intentionally not
tracked, because that lane can render real secret material.

Use:

- `just render` to refresh tracked artifacts
- `just render-all` to also materialize local-only private outputs
- `just validate` to check tracked artifacts are current and do not embed secret
  material
