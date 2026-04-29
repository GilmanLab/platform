package incusos

@go(incusos)

// NonEmptyString is a required string value that must not be empty.
#NonEmptyString: (string & !="") | error("must be a non-empty string")

// HTTPURL is an HTTP or HTTPS URL.
#HTTPURL: (#NonEmptyString & =~"^https?://") | error("must be an HTTP or HTTPS URL")

// ImageFormat is the compressed image artifact format produced by the build.
#ImageFormat: *"img.gz" | "img.gz" | error("format must be img.gz")

// ImageVersion selects the upstream IncusOS image version.
#ImageVersion: *"latest" | #NonEmptyString | error("version must be latest or a non-empty version string")

// ImageSize is the expected image size written into the IncusOS seed metadata.
#ImageSize: (#NonEmptyString & =~"^[1-9][0-9]*[KMGT]$") | error("size must be a positive whole number with a K, M, G, or T suffix")

// SeedOffset is the byte offset where seed data is written in the image.
#SeedOffset: (int & >0) | error("seed offset must be a positive integer")

// SecretRef points at a field in a SOPS-managed secret document.
#SecretRef: {
	@go(SecretRef)

	// Path is the repo-relative path to the SOPS-managed secret document.
	path!: #NonEmptyString
	// Field is the field name inside the secret document.
	field!: #NonEmptyString
}

// SecretString models a string value sourced from an external secret.
#SecretString: {
	@go(SecretString)

	// SecretRef identifies the secret field that supplies the value.
	secretRef!: #SecretRef
}

// ImageSource describes where to download the upstream IncusOS image from.
#ImageSource: {
	@go(ImageSource)

	// IndexURL is the upstream stream index URL used to discover image metadata.
	indexURL!: #HTTPURL @go(IndexURL)
	// BaseURL is the upstream base URL used to download image artifacts.
	baseURL!: #HTTPURL @go(BaseURL)
	// Channel selects the upstream IncusOS release stream.
	channel: *"stable" | #NonEmptyString | error("channel must be stable or a non-empty channel name")
	// Arch selects the upstream IncusOS image architecture.
	arch: *"x86_64" | #NonEmptyString | error("architecture must be x86_64 or a non-empty architecture name")
	// Version selects the upstream IncusOS image version.
	version: #ImageVersion
}

// ImageOutput describes the local image artifact produced by the build.
#ImageOutput: {
	@go(ImageOutput)

	// Dir is the output directory for the generated image artifact.
	dir!: #NonEmptyString
	// ArtifactName is the output file name for the generated image artifact.
	artifactName!: #NonEmptyString @go(ArtifactName)
	// Size is the expected image size written into the IncusOS seed metadata.
	size!: #ImageSize
	// Format is the compressed image artifact format.
	format: #ImageFormat
}

// Application is an IncusOS application entry to enable during first boot.
#Application: {
	@go(Application)

	// Name is the IncusOS application name.
	name!: #NonEmptyString
}

// ApplicationsSeed is the contents of applications.yaml in the image seed.
#ApplicationsSeed: {
	@go(ApplicationsSeed)

	// Version is the applications.yaml schema version.
	version: *"1" | "1" | error("applications seed version must be 1")
	// Applications is the non-empty set of IncusOS applications to enable.
	applications: ([...#Application] & [_, ...]) @go(,type=[]Application)
}

// TrustedClientCertificate is an exact trusted Incus client certificate entry.
#TrustedClientCertificate: {
	@go(TrustedClientCertificate)

	// Name is the stable name for the trusted client certificate.
	name!: #NonEmptyString
	// Type is the Incus certificate type.
	type: *"client" | "client" | error("trusted certificate type must be client")
	// Certificate is the PEM-encoded certificate loaded from secrets.
	certificate!: #SecretString
}

// IncusPreseed is the Incus preseed payload rendered into incus.yaml.
#IncusPreseed: {
	@go(IncusPreseed)

	// Config contains optional Incus daemon configuration keys.
	config?: [string]: string
	// Certificates is the non-empty set of trusted client certificates to seed.
	certificates: ([...#TrustedClientCertificate] & [_, ...]) @go(,type=[]TrustedClientCertificate)
}

// IncusSeed is the contents of incus.yaml in the image seed.
#IncusSeed: {
	@go(IncusSeed)

	// Version is the incus.yaml schema version.
	version: *"1" | "1" | error("Incus seed version must be 1")
	// ApplyDefaults controls whether IncusOS applies its default preseed values.
	apply_defaults: *true | bool @go(ApplyDefaults)
	// Preseed is the Incus server configuration applied on first boot.
	preseed!: #IncusPreseed
}

// Seed describes the full IncusOS seed payload written into the image.
#Seed: {
	@go(Seed)

	// Offset is the byte offset where seed data is written in the image.
	offset!: #SeedOffset
	// Applications is the applications.yaml seed payload.
	applications!: #ApplicationsSeed
	// Incus is the incus.yaml seed payload.
	incus!: #IncusSeed
}

// ImageBuild is the top-level IncusOS image download, verification, and seed contract.
#ImageBuild: {
	@go(ImageBuild)

	// Name is the stable build name used in logs and generated metadata.
	name!: #NonEmptyString
	// Source describes the upstream IncusOS image to download.
	source!: #ImageSource
	// Output describes the local image artifact produced by the build.
	output!: #ImageOutput
	// Seed describes the first-boot configuration written into the image.
	seed!: #Seed
}
