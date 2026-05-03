package talos

@go(talos)

// NonEmptyString is a required string value that must not be empty.
#NonEmptyString: (string & !="") | error("must be a non-empty string")

// HTTPURL is an HTTP or HTTPS URL.
#HTTPURL: (#NonEmptyString & =~"^https?://") | error("must be an HTTP or HTTPS URL")

// RelativePath is a repository- or config-relative path.
#RelativePath: (#NonEmptyString & !~"^/" & !~"(^|/)\\.\\.(/|$)" & !~"\\\\") |
	error("path must be relative and use forward slashes")

// ArtifactName is a local artifact filename, not a path.
#ArtifactName: (#NonEmptyString & !~"/" & !~"\\\\" & =~"\\.img$") |
	error("artifact name must be an .img filename")

// SchematicID is an Image Factory schematic ID.
#SchematicID: =~"^[a-f0-9]{64}$" | error("schematic ID must be a 64-character lowercase hex string")

// TalosVersion selects an exact Talos Linux release.
#TalosVersion: =~"^v[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?(\\+[0-9A-Za-z.-]+)?$" |
	error("Talos version must be an exact release like v1.13.0")

// Architecture selects the Talos image architecture.
#Architecture: *"amd64" | "amd64" | "arm64" | error("architecture must be amd64 or arm64")

// Platform selects the Talos image platform.
#Platform: *"nocloud" | "nocloud" | error("platform must be nocloud")

// SourceArtifact selects the Image Factory artifact to download.
#SourceArtifact: *"raw.xz" | "raw.xz" | error("source artifact must be raw.xz")

// OutputFormat selects the local artifact format produced by the build.
#OutputFormat: *"img" | "img" | error("format must be img")

// ConfigDelivery selects how the Talos machine configuration is delivered.
#ConfigDelivery: *"nocloud-cidata" | "nocloud-cidata" |
	error("config delivery must be nocloud-cidata")

// DNSLabel is a single RFC 1123-style hostname label.
#DNSLabel: (#NonEmptyString & =~"^[a-z0-9]([-a-z0-9]*[a-z0-9])?$") |
	error("hostname must be a lowercase DNS label")

// FileInput points at a local file consumed by the build.
#FileInput: {
	@go(FileInput)

	// Path is a relative path from the build config directory.
	path!: #RelativePath
}

// ImageSource describes the Talos Image Factory asset to download.
#ImageSource: {
	@go(ImageSource)

	// FactoryURL is the Image Factory base URL.
	factoryURL: *"https://factory.talos.dev" | #HTTPURL @go(FactoryURL)
	// Version is the exact Talos Linux release to download.
	version!: #TalosVersion
	// SchematicID identifies the Image Factory schematic.
	schematicID: *"376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba" | #SchematicID @go(SchematicID)
	// Platform selects the Talos platform image.
	platform: #Platform
	// Arch selects the Talos image architecture.
	arch: #Architecture
	// Artifact selects the compressed Image Factory disk artifact.
	artifact: #SourceArtifact
}

// NoCloudMetaData describes the meta-data file in the NoCloud cidata image.
#NoCloudMetaData: {
	@go(NoCloudMetaData)

	// LocalHostname is the VM hostname advertised through NoCloud.
	localHostname!: #DNSLabel @go(LocalHostname)
	// InstanceID identifies this NoCloud instance. It defaults to localHostname.
	instanceID: *localHostname | #NonEmptyString @go(InstanceID)
}

// MachineConfig describes Talos machine configuration delivery.
#MachineConfig: {
	@go(MachineConfig)

	// Delivery controls how config files are packaged for Talos.
	delivery: #ConfigDelivery
	// UserData is the Talos machine config written as NoCloud user-data.
	userData!: #FileInput @go(UserData)
	// MetaData is the NoCloud meta-data payload.
	metaData!: #NoCloudMetaData @go(MetaData)
	// NetworkConfig optionally points at a NoCloud network-config file.
	networkConfig?: #FileInput @go(NetworkConfig)
}

// ImageOutput describes the local artifacts produced by the build.
#ImageOutput: {
	@go(ImageOutput)

	// Dir is the output directory for generated artifacts.
	dir!: #NonEmptyString
	// Format is the local artifact format.
	format: #OutputFormat
	// BootArtifactName is the Talos boot disk IMG filename.
	bootArtifactName!: #ArtifactName @go(BootArtifactName)
	// ConfigArtifactName is the NoCloud cidata IMG filename.
	configArtifactName!: #ArtifactName @go(ConfigArtifactName)
}

// ImageBuild is the top-level Talos image download and config packaging contract.
#ImageBuild: {
	@go(ImageBuild)

	// Name is the stable build name used in logs and generated metadata.
	name!: #NonEmptyString
	// Source describes the upstream Talos image to download.
	source!: #ImageSource
	// Config describes Talos machine configuration delivery.
	config!: #MachineConfig
	// Output describes the local artifacts produced by the build.
	output!: #ImageOutput
}
