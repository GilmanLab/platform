package platformrgds

bundle: {
	name:     "platform-rgds"
	package:  "rgds/platform"
	artifact: "ghcr.io/gilmanlab/platform/rgds/platform-rgds"
	rgdName:  "platform"
	api: {
		group:   "platform.gilman.io"
		version: "v1alpha1"
		kind:    "Platform"
		scope:   "Cluster"
	}
}
