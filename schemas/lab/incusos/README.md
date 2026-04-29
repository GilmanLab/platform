# IncusOS Schema

This package defines the CUE schema for IncusOS image build configuration. It is
used by platform tooling, including `labctl`, to validate and consume the image
source, output artifact, and seed payload contract.

The CUE schema is the source of truth; Go types are generated from `schema.cue`.

## Example

```cue
package images

import incusos "github.com/gilmanlab/platform/schemas/lab/incusos"

build: incusos.#ImageBuild & {
	name: "incusos-operation-first-node"
	source: {
		indexURL: "https://images.linuxcontainers.org/os/index.json"
		baseURL:  "https://images.linuxcontainers.org/os"
		channel:  "stable"
		arch:     "x86_64"
		version:  "latest"
	}
	output: {
		dir:          ".state/images"
		artifactName: "incusos-operation-first-node-x86_64.img.gz"
		size:         "50G"
		format:       "img.gz"
	}
	seed: {
		offset: 2148532224
		applications: {
			applications: [{name: "incus"}]
		}
		incus: {
			preseed: {
				certificates: [{
					name: "bootstrap-client"
					certificate: secretRef: {
						path:  "compute/incusos/bootstrap-client.sops.yaml"
						field: "client_crt_pem"
					}
				}]
			}
		}
	}
}
```
