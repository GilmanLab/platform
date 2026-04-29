package incusos

@go(incusos)

#NonEmptyString: string & !=""

#HTTPURL: #NonEmptyString & =~"^https?://"

#ImageFormat: "img.gz"

#ImageVersion: *"latest" | #NonEmptyString

#ImageSize: #NonEmptyString & =~"^[1-9][0-9]*[KMGT]$"

#SeedOffset: int & >0

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
	channel:   *"stable" | #NonEmptyString
	arch:      *"x86_64" | #NonEmptyString
	version:   #ImageVersion
}

#ImageOutput: {
	@go(ImageOutput)

	dir!:          #NonEmptyString
	artifactName!: #NonEmptyString @go(ArtifactName)
	size!:         #ImageSize
	format:        *"img.gz" | #ImageFormat
}

#Application: {
	@go(Application)

	name!: #NonEmptyString
}

#ApplicationsSeed: {
	@go(ApplicationsSeed)

	version: *"1" | "1"
	applications: ([...#Application] & [_, ...]) @go(,type=[]Application)
}

#TrustedClientCertificate: {
	@go(TrustedClientCertificate)

	name!:        #NonEmptyString
	type:         *"client" | "client"
	certificate!: #SecretString
}

#IncusPreseed: {
	@go(IncusPreseed)

	config?: [string]: string
	certificates: ([...#TrustedClientCertificate] & [_, ...]) @go(,type=[]TrustedClientCertificate)
}

#IncusSeed: {
	@go(IncusSeed)

	version:        *"1" | "1"
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
