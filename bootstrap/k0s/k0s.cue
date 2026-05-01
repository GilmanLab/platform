package k0s

import (
	"encoding/yaml"
	"strings"
)

cfgPodCIDR:             string @tag(pod_cidr)
cfgServiceCIDR:         string @tag(service_cidr)
cfgPublicIP:            string @tag(public_ip)
cfgArtifactsFileServer: string @tag(artifacts_file_server)
cfgDHCPBindInterface:   string @tag(dhcp_bind_interface)
cfgHookOSArch:          string @tag(hookos_arch)
cfgHookOSKernelVersion: string @tag(hookos_kernel_version)
cfgHookOSExtension:     string @tag(hookos_extension)
cfgDHCPMode:            string @tag(dhcp_mode)
cfgTinkServerAddrPort:  string @tag(tink_server_addr_port)
cfgOSIEURL:             string @tag(osie_url)
cfgTrustedProxiesCSV:   string @tag(trusted_proxies)

cfgTrustedProxies: [for proxy in strings.Split(cfgTrustedProxiesCSV, ",") if strings.TrimSpace(proxy) != "" {
	strings.TrimSpace(proxy)
}]

#CertManagerValues: {
	crds: {
		enabled: true
	}
}

#TinkerbellValues: {
	trustedProxies:      cfgTrustedProxies
	publicIP:            cfgPublicIP
	artifactsFileServer: cfgArtifactsFileServer
	service: {
		type: "ClusterIP"
	}
	deployment: {
		additionalEnvs: [{
			name:  "TINKERBELL_BIND_ADDRESS"
			value: cfgPublicIP
		}]
		hostNetwork: true
		init: {
			enabled: false
		}
		strategy: {
			type: "Recreate"
		}
		envs: {
			globals: {
				bindAddr: "0.0.0.0"
			}
			smee: {
				dhcpEnabled:                  false
				dhcpMode:                     cfgDHCPMode
				dhcpBindInterface:            cfgDHCPBindInterface
				dhcpIPForPacket:              cfgPublicIP
				dhcpTftpIP:                   cfgPublicIP
				dhcpSyslogIP:                 cfgPublicIP
				dhcpIpxeHttpBinaryHost:       cfgPublicIP
				dhcpIpxeHttpScriptHost:       cfgPublicIP
				ipxeScriptTinkServerAddrPort: cfgTinkServerAddrPort
				ipxeHttpScriptOsieURL:        cfgOSIEURL
				tftpServerBindAddr:           "0.0.0.0"
				ipxeHttpScriptExtraKernelArgs: ["ip=dhcp"]
				ipxeHttpScriptRetries: 5
			}
			tinkServer: {
				bindAddr: "0.0.0.0"
			}
		}
	}
	optional: {
		osie: {
			hostNetwork: true
			service: {
				type: "ClusterIP"
			}
		}
		hookos: {
			enabled:       true
			arch:          cfgHookOSArch
			kernelVersion: cfgHookOSKernelVersion
			extension:     cfgHookOSExtension
		}
		kubevip: {
			enabled: false
		}
	}
}

output: {
	apiVersion: "k0s.k0sproject.io/v1beta1"
	kind:       "ClusterConfig"
	metadata: {
		name:      "bootstrap-k0s"
		namespace: "kube-system"
	}
	spec: {
		network: {
			podCIDR:     cfgPodCIDR
			serviceCIDR: cfgServiceCIDR
		}
		extensions: {
			helm: {
				concurrencyLevel: 1
				charts: [
					{
						name:      "cert-manager"
						chartname: "/opt/bootstrap/charts/cert-manager.tgz"
						namespace: "cert-manager"
						order:     0
						values:    yaml.Marshal(#CertManagerValues)
					},
					{
						name:      "cluster-api-operator"
						chartname: "/opt/bootstrap/charts/cluster-api-operator.tgz"
						namespace: "capi-operator-system"
						order:     10
					},
					{
						name:      "tinkerbell"
						chartname: "/opt/bootstrap/charts/tinkerbell.tgz"
						namespace: "tinkerbell"
						order:     20
						values:    yaml.Marshal(#TinkerbellValues)
					},
				]
			}
		}
	}
}
