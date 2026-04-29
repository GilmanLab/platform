package incusos

@go(incusos)

#NonEmptyString: (string & !="") | error("must be a non-empty string")

#HTTPURL: (#NonEmptyString & =~"^https?://") | error("must be an HTTP or HTTPS URL")

#ImageFormat: *"img.gz" | "img.gz" | error("format must be img.gz")

#ImageVersion: *"latest" | #NonEmptyString | error("version must be latest or a non-empty version string")

#ImageSize: (#NonEmptyString & =~"^[1-9][0-9]*[KMGT]$") | error("size must be a positive whole number with a K, M, G, or T suffix")

#SeedOffset: (int & >0) | error("seed offset must be a positive integer")

#SecretRef: {
	@go(SecretRef)

	path!:  #NonEmptyString
	field!: #NonEmptyString
}

#SecretString: {
	@go(SecretString)

	secretRef!: #SecretRef
}

#ImageSource: {
	@go(ImageSource)

	indexURL!: #HTTPURL @go(IndexURL)
	baseURL!:  #HTTPURL @go(BaseURL)
	channel:   *"stable" | #NonEmptyString | error("channel must be stable or a non-empty channel name")
	arch:      *"x86_64" | #NonEmptyString | error("architecture must be x86_64 or a non-empty architecture name")
	version:   #ImageVersion
}

#ImageOutput: {
	@go(ImageOutput)

	dir!:          #NonEmptyString
	artifactName!: #NonEmptyString @go(ArtifactName)
	size!:         #ImageSize
	format:        #ImageFormat
}

#Application: {
	@go(Application)

	name!: #NonEmptyString
}

#ApplicationsSeed: {
	@go(ApplicationsSeed)

	version: *"1" | "1" | error("applications seed version must be 1")
	applications: ([...#Application] & [_, ...]) @go(,type=[]Application)
}

#TrustedClientCertificate: {
	@go(TrustedClientCertificate)

	name!:        #NonEmptyString
	type:         *"client" | "client" | error("trusted certificate type must be client")
	certificate!: #SecretString
}

#IncusPreseed: {
	@go(IncusPreseed)

	config?: [string]: string
	certificates: ([...#TrustedClientCertificate] & [_, ...]) @go(,type=[]TrustedClientCertificate)
}

#IncusSeed: {
	@go(IncusSeed)

	version:        *"1" | "1" | error("Incus seed version must be 1")
	apply_defaults: *true | bool @go(ApplyDefaults)
	preseed!:       #IncusPreseed
}

#Seed: {
	@go(Seed)

	offset!:       #SeedOffset
	applications!: #ApplicationsSeed
	incus!:        #IncusSeed
}

#ImageBuild: {
	@go(ImageBuild)

	name!:   #NonEmptyString
	source!: #ImageSource
	output!: #ImageOutput
	seed!:   #Seed
}
