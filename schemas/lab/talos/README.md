# Talos Schema

This package defines the CUE schema for Talos image build configuration. It is
used by platform tooling, including `labctl`, to validate the Image Factory
source, Talos machine configuration delivery, and generated local artifacts for
the temporary bootstrap cluster.

The CUE schema is the source of truth; Go types are generated from `schema.cue`.

## Example

The minimum valid configuration relies on schema defaults for the Image
Factory URL, schematic ID, platform, architecture, artifact, output format,
output directory, and the boot/cidata artifact filenames:

```cue
package images

import talos "github.com/gilmanlab/platform/schemas/lab/talos"

build: talos.#ImageBuild & {
	name: "bootstrap-talos-controlplane"
	source: version: "v1.13.0"
	config: {
		userData: path: "controlplane.yaml"
		metaData: localHostname: "bootstrap-controlplane-1"
	}
}
```

Override any default by setting the field explicitly. For example, to land
artifacts in a different directory with bespoke filenames:

```cue
build: output: {
	dir:                ".state/images/talos-bootstrap"
	bootArtifactName:   "talos-bootstrap-amd64.img"
	configArtifactName: "talos-bootstrap-cidata.img"
}
```
