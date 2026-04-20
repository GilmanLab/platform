package platformrgds

import (
	dnscap "github.com/gilmanlab/platform/rgds/platform/capabilities/dns"
	tlscap "github.com/gilmanlab/platform/rgds/platform/capabilities/tls"
)

output: {
	apiVersion: "kro.run/v1alpha1"
	kind:       "ResourceGraphDefinition"
	metadata: {
		name: bundle.rgdName
	}
	spec: {
		schema: {
			apiVersion: bundle.api.version
			kind:       bundle.api.kind
			group:      bundle.api.group
			scope:      bundle.api.scope
			spec: {
				dns: dnscap.Spec
				tls: tlscap.Spec
			}
		}
		resources: []
	}
}
