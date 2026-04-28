package providers

capiVersion:                   string @tag(capi_version)
capnVersion:                   string @tag(capn_version)
talosBootstrapProviderVersion: string @tag(talos_bootstrap_provider_version)
talosControlPlaneVersion:      string @tag(talos_control_plane_provider_version)

output: {
	apiVersion: "v1"
	kind:       "List"
	items: [
		{
			apiVersion: "operator.cluster.x-k8s.io/v1alpha2"
			kind:       "CoreProvider"
			metadata: {
				name:      "cluster-api"
				namespace: "capi-system"
			}
			spec: {
				version: capiVersion
				fetchConfig: {
					url: "https://github.com/kubernetes-sigs/cluster-api/releases/download/\(capiVersion)/core-components.yaml"
				}
			}
		},
		{
			apiVersion: "operator.cluster.x-k8s.io/v1alpha2"
			kind:       "BootstrapProvider"
			metadata: {
				name:      "talos"
				namespace: "cabpt-system"
			}
			spec: {
				version: talosBootstrapProviderVersion
				fetchConfig: {
					url: "https://github.com/siderolabs/cluster-api-bootstrap-provider-talos/releases/download/\(talosBootstrapProviderVersion)/bootstrap-components.yaml"
				}
			}
		},
		{
			apiVersion: "operator.cluster.x-k8s.io/v1alpha2"
			kind:       "ControlPlaneProvider"
			metadata: {
				name:      "talos"
				namespace: "cacppt-system"
			}
			spec: {
				version: talosControlPlaneVersion
				fetchConfig: {
					url: "https://github.com/siderolabs/cluster-api-control-plane-provider-talos/releases/download/\(talosControlPlaneVersion)/control-plane-components.yaml"
				}
			}
		},
		{
			apiVersion: "operator.cluster.x-k8s.io/v1alpha2"
			kind:       "InfrastructureProvider"
			metadata: {
				name:      "incus"
				namespace: "capn-system"
			}
			spec: {
				version: capnVersion
				fetchConfig: {
					url: "https://github.com/lxc/cluster-api-provider-incus/releases/download/\(capnVersion)/infrastructure-components.yaml"
				}
			}
		},
	]
}
