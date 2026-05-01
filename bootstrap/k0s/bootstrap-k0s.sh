#!/bin/sh
set -eu

provider_namespaces_src="/opt/bootstrap/providers/namespaces.yaml"
provider_src="/opt/bootstrap/providers/providers.yaml"
provider_dst_dir="/var/lib/k0s/manifests/providers"
provider_namespaces_dst="${provider_dst_dir}/00-namespaces.yaml"
provider_dst="${provider_dst_dir}/10-providers.yaml"
k0s_source="/opt/bootstrap/k0s.cue"
k0s_config="/etc/k0s/k0s.yaml"

require_env() {
  name="$1"
  eval "value=\${$name:-}"
  if [ -z "$value" ]; then
    echo "missing required environment variable: $name" >&2
    exit 1
  fi
}

render_k0s_config() {
  require_env TINKERBELL_PUBLIC_IP
  require_env TINKERBELL_ARTIFACTS_FILE_SERVER
  require_env TINKERBELL_DHCP_BIND_INTERFACE

  K0S_POD_CIDR="${K0S_POD_CIDR:-10.244.0.0/16}"
  K0S_SERVICE_CIDR="${K0S_SERVICE_CIDR:-10.96.0.0/12}"
  TINKERBELL_TRUSTED_PROXIES="${TINKERBELL_TRUSTED_PROXIES:-$K0S_POD_CIDR,$K0S_SERVICE_CIDR}"
  TINKERBELL_HOOKOS_ARCH="${TINKERBELL_HOOKOS_ARCH:-x86_64}"
  TINKERBELL_HOOKOS_KERNEL_VERSION="${TINKERBELL_HOOKOS_KERNEL_VERSION:-6.6}"
  TINKERBELL_HOOKOS_EXTENSION="${TINKERBELL_HOOKOS_EXTENSION:-tar.gz}"
  TINKERBELL_DHCP_MODE="${TINKERBELL_DHCP_MODE:-reservation}"
  TINKERBELL_TINK_SERVER_ADDR_PORT="${TINKERBELL_TINK_SERVER_ADDR_PORT:-$TINKERBELL_PUBLIC_IP:42113}"
  TINKERBELL_OSIE_URL="${TINKERBELL_OSIE_URL:-http://$TINKERBELL_PUBLIC_IP:7173}"

  cue export "$k0s_source" \
    -e output \
    --out yaml \
    -t pod_cidr="$K0S_POD_CIDR" \
    -t service_cidr="$K0S_SERVICE_CIDR" \
    -t public_ip="$TINKERBELL_PUBLIC_IP" \
    -t artifacts_file_server="$TINKERBELL_ARTIFACTS_FILE_SERVER" \
    -t dhcp_bind_interface="$TINKERBELL_DHCP_BIND_INTERFACE" \
    -t hookos_arch="$TINKERBELL_HOOKOS_ARCH" \
    -t hookos_kernel_version="$TINKERBELL_HOOKOS_KERNEL_VERSION" \
    -t hookos_extension="$TINKERBELL_HOOKOS_EXTENSION" \
    -t dhcp_mode="$TINKERBELL_DHCP_MODE" \
    -t tink_server_addr_port="$TINKERBELL_TINK_SERVER_ADDR_PORT" \
    -t osie_url="$TINKERBELL_OSIE_URL" \
    -t trusted_proxies="$TINKERBELL_TRUSTED_PROXIES" \
    >"$k0s_config"
}

render_k0s_config
mkdir -p "$provider_dst_dir"
cp "$provider_namespaces_src" "$provider_namespaces_dst"

echo "starting k0s bootstrap controller"
k0s controller --config "$k0s_config" --single --ignore-pre-flight-checks &
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
    cp "$provider_src" "$provider_dst"
    break
  fi

  sleep 2
done

wait "$k0s_pid"
