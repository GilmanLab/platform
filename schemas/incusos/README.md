# IncusOS Schema

This package defines the CUE contract for building a seeded IncusOS image for
lab bootstrap use. It covers the first `labctl` image-build slice: download the
upstream image, verify it from upstream metadata, write the seed payload, and
produce a `.img.gz` artifact.

The package intentionally does not model VyOS staging, image hosting, OCI
publication, or long-term Incus CA trust. Seeded trust starts with exact trusted
client certificate entries whose certificate material is resolved from external
SOPS-managed secrets.

The CUE schema is the source of truth. The Go types in this package are
generated from `schema.cue`.

## Example

```cue
package images

import incusos "github.com/gilmanlab/platform/schemas/incusos"

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
