# bootstrap-k0s

`bootstrap-k0s` builds the temporary `k0s` bootstrap image for the lab's VyOS-hosted management cluster. It brings up a single-node `k0s` control plane, renders the Tinkerbell provisioning configuration from a small runtime contract using CUE, and then hands off the Cluster API provider resources only after the Cluster API Operator is actually ready.

This image is for the bootstrap management plane only. It is not the long-term platform cluster shape.

## What It Installs

- `cert-manager`
- `cluster-api-operator`
- `tinkerbell`
- `CoreProvider` for Cluster API
- `BootstrapProvider` for Talos
- `ControlPlaneProvider` for Talos
- `InfrastructureProvider` for Incus

The first three are installed through `k0s` `spec.extensions.helm`. The provider CRs are rendered from CUE at build time and staged into the k0s Manifest Deployer only after the operator CRDs and webhook are ready.

## Runtime Contract

The image is built once in `platform` and configured at runtime by the
router-side consumer in `infra`.

Required environment variables:

- `TINKERBELL_PUBLIC_IP`
- `TINKERBELL_ARTIFACTS_FILE_SERVER`
- `TINKERBELL_DHCP_BIND_INTERFACE`

Optional environment variables:

- `TINKERBELL_TRUSTED_PROXIES` defaults to `10.244.0.0/16,10.96.0.0/12`
- `K0S_POD_CIDR` defaults to `10.244.0.0/16`
- `K0S_SERVICE_CIDR` defaults to `10.96.0.0/12`
- `TINKERBELL_HOOKOS_ARCH` defaults to `x86_64`
- `TINKERBELL_HOOKOS_KERNEL_VERSION` defaults to `6.6`
- `TINKERBELL_HOOKOS_EXTENSION` defaults to `tar.gz`
- `TINKERBELL_DHCP_MODE` defaults to `reservation`

The intended real-lab consumer passes:

```text
TINKERBELL_PUBLIC_IP=10.10.20.1
TINKERBELL_ARTIFACTS_FILE_SERVER=http://10.10.20.1:7173
TINKERBELL_DHCP_BIND_INTERFACE=eth1.20
```

The VyOS-hosted bootstrap path leaves DHCP with VyOS/Kea. Tinkerbell's DHCP
listener is disabled, while its HTTP, TFTP, gRPC, and SSH listeners bind to
`TINKERBELL_PUBLIC_IP`.

For local smoke runs, set `TINKERBELL_DHCP_BIND_INTERFACE` to an interface that
actually exists inside the disposable bootstrap container, such as `eth0`.

## File Layout

- [Dockerfile](./Dockerfile) builds the runtime image and packages the startup artifacts.
- [k0s.cue](./k0s.cue) is the tracked `k0s` config source of truth.
- [bootstrap-k0s.sh](./bootstrap-k0s.sh) exports the final `k0s` config from CUE at runtime, starts `k0s`, and delays the provider-manifest handoff until the operator is ready.
- [manifests/providers/namespaces.yaml](./manifests/providers/namespaces.yaml) seeds the provider namespaces immediately.
- [manifests/providers/providers.cue](./manifests/providers/providers.cue) is the source of truth for the provider CRs.
- [moon.yml](./moon.yml) defines the local lint and image build checks.
- [VERSION](./VERSION) is the release-please version source for the published image tag.

## Build

From the repository root:

```sh
docker build -t bootstrap-k0s:test -f bootstrap/k0s/Dockerfile .
```

Lint the Dockerfile:

```sh
moon run bootstrap-k0s:lint --summary minimal
```

## Local Smoke Test

Start a disposable local bootstrap cluster:

```sh
docker volume create bootstrap-k0s-data
docker volume create bootstrap-k0s-pods

docker run -d \
  --name bootstrap-k0s-smoke \
  --hostname bootstrap-k0s-smoke \
  --privileged \
  --tmpfs /run \
  -v bootstrap-k0s-data:/var/lib/k0s \
  -v bootstrap-k0s-pods:/var/log/pods \
  -p 6443:6443 \
  -e TINKERBELL_PUBLIC_IP=10.10.20.1 \
  -e TINKERBELL_ARTIFACTS_FILE_SERVER=http://10.10.20.1:7173 \
  -e TINKERBELL_DHCP_BIND_INTERFACE=eth0 \
  bootstrap-k0s:test
```

After the startup settles, inspect the bootstrap controllers:

```sh
docker exec bootstrap-k0s-smoke sh -lc '
  k0s kubectl get coreproviders,bootstrapproviders,controlplaneproviders,infrastructureproviders -A
  echo
  k0s kubectl get deploy -A
'
```

The validated local outcome is:

- `cluster-api-operator`, `cert-manager`, and `tinkerbell` are `1/1` Ready
- Tinkerbell runs in host-networked provisioning mode with HookOS enabled
- provider CRs report `READY=True`
- provider controller deployments in `capi-system`, `cabpt-system`, `cacppt-system`, and `capn-system` are `1/1` Ready

Clean up:

```sh
docker rm -f bootstrap-k0s-smoke
docker volume rm bootstrap-k0s-data bootstrap-k0s-pods
```

## Release

`bootstrap-k0s` is a release-please-managed subproject.

- `VERSION` is the source of truth for the current released version.
- release tags are `bootstrap-k0s-v*`.
- published images land at `ghcr.io/gilmanlab/platform/bootstrap-k0s:<version>`.
- `infra` should consume an exact released tag, not `latest`.

## Notes

- The final runtime `k0s` config is exported from `k0s.cue` with the bundled `cue` CLI before `k0s` starts.
- Provider CRs are rendered with CUE during the image build, and the runtime `k0s` config is rendered with CUE at container startup.
- The delayed handoff exists because the operator webhook is not ready early enough for a naive first-pass Manifest Deployer apply.

## Contributing

Follow the repository-wide guidance in [CONTRIBUTING.md](../../CONTRIBUTING.md).

## License

No component-specific license file is defined in this directory.
