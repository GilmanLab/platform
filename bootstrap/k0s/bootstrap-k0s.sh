#!/bin/sh
set -eu

provider_src="/opt/bootstrap/providers/providers.yaml"
provider_dst_dir="/var/lib/k0s/manifests/providers"
provider_dst="${provider_dst_dir}/10-providers.yaml"

echo "starting k0s bootstrap controller"
k0s controller --single --ignore-pre-flight-checks &
k0s_pid=$!

cleanup() {
  if kill -0 "$k0s_pid" 2>/dev/null; then
    kill "$k0s_pid" 2>/dev/null || true
  fi
}

trap cleanup INT TERM

while :; do
  if ! kill -0 "$k0s_pid" 2>/dev/null; then
    wait "$k0s_pid"
    exit $?
  fi

  if ! k0s kubectl get crd \
    coreproviders.operator.cluster.x-k8s.io \
    bootstrapproviders.operator.cluster.x-k8s.io \
    controlplaneproviders.operator.cluster.x-k8s.io \
    infrastructureproviders.operator.cluster.x-k8s.io >/dev/null 2>&1; then
    sleep 2
    continue
  fi

  if ! k0s kubectl -n capi-operator-system wait \
    --for=condition=Available \
    deployment/cluster-api-operator \
    --timeout=5s >/dev/null 2>&1; then
    sleep 2
    continue
  fi

  if k0s kubectl --request-timeout=5s apply --dry-run=server -f "$provider_src" >/dev/null 2>&1; then
    echo "operator API is ready; handing provider manifests to k0s manifest deployer"
    mkdir -p "$provider_dst_dir"
    cp "$provider_src" "$provider_dst"
    break
  fi

  sleep 2
done

wait "$k0s_pid"
