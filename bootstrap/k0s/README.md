# bootstrap-k0s

`bootstrap-k0s` builds the temporary `k0s` bootstrap image for the lab's VyOS-hosted management cluster. It brings up a single-node `k0s` control plane, installs the bootstrap charts that have native Helm packaging, and then hands off the Cluster API provider resources only after the Cluster API Operator is actually ready.

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

## File Layout

- [Dockerfile](./Dockerfile) builds the runtime image and packages the startup artifacts.
- [k0s.yaml](./k0s.yaml) is the static `k0s` cluster configuration.
- [bootstrap-k0s.sh](./bootstrap-k0s.sh) starts `k0s` and delays the provider-manifest handoff until the operator is ready.
- [manifests/providers/namespaces.yaml](./manifests/providers/namespaces.yaml) seeds the provider namespaces immediately.
- [manifests/providers/providers.cue](./manifests/providers/providers.cue) is the source of truth for the provider CRs.
- [moon.yml](./moon.yml) defines the local Dockerfile lint task.

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
- provider CRs report `READY=True`
- provider controller deployments in `capi-system`, `cabpt-system`, `cacppt-system`, and `capn-system` are `1/1` Ready

Clean up:

```sh
docker rm -f bootstrap-k0s-smoke
docker volume rm bootstrap-k0s-data bootstrap-k0s-pods
```

## Notes

- This directory intentionally keeps `k0s.yaml` static.
- Provider CRs are rendered with CUE during the image build, not with ad hoc shell templating.
- The delayed handoff exists because the operator webhook is not ready early enough for a naive first-pass Manifest Deployer apply.

## Contributing

Follow the repository-wide guidance in [CONTRIBUTING.md](../../CONTRIBUTING.md).

## License

No component-specific license file is defined in this directory.
