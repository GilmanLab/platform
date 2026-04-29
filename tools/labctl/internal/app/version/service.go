package version

// Info describes the labctl build version.
type Info struct {
	// Version is the release version embedded into the labctl binary.
	Version string `json:"version"`
}

// Service returns version information for the current labctl binary.
type Service struct {
	info Info
}

// NewService constructs a Service from compiler-provided build metadata.
func NewService(value string) Service {
	if value == "" {
		value = "dev"
	}

	return Service{
		info: Info{
			Version: value,
		},
	}
}

// Info returns the current labctl build version.
func (s Service) Info() Info {
	return s.info
}
