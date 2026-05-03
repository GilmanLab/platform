package talosimage

import (
	"context"
	"io"
	"io/fs"

	schematalos "github.com/gilmanlab/platform/schemas/lab/talos"
)

// Request describes a Talos image build request.
type Request struct {
	// Config is the validated Talos image build configuration.
	Config schematalos.ImageBuild
	// BaseDir resolves relative paths in Config.
	BaseDir string
}

// Result describes the built Talos image artifacts.
type Result struct {
	// Name is the stable build name from the input configuration.
	Name string `json:"name"`
	// BootArtifactPath is the local path to the Talos boot disk image.
	BootArtifactPath string `json:"bootArtifactPath"`
	// ConfigArtifactPath is the local path to the NoCloud cidata image.
	ConfigArtifactPath string `json:"configArtifactPath"`
	// SourceVersion is the selected upstream Talos Linux version.
	SourceVersion string `json:"sourceVersion"`
	// SourceURL is the selected upstream Talos Image Factory artifact URL.
	SourceURL string `json:"sourceURL"`
	// SourceSchematicID is the Image Factory schematic ID used for the artifact.
	SourceSchematicID string `json:"sourceSchematicID"`
	// Platform is the Talos Image Factory platform.
	Platform string `json:"platform"`
	// Arch is the Talos Image Factory architecture.
	Arch string `json:"arch"`
	// Format is the local artifact format.
	Format string `json:"format"`
}

// Dependencies groups external ports needed to build Talos image artifacts.
type Dependencies struct {
	// Upstream downloads Talos Image Factory artifacts.
	Upstream Upstream
	// Files reads and writes local build artifacts.
	Files FileSystem
	// ConfigDisk builds NoCloud cidata images.
	ConfigDisk ConfigDiskBuilder
}

// Upstream downloads Talos Image Factory artifact bytes.
type Upstream interface {
	Download(ctx context.Context, url string) (io.ReadCloser, error)
}

// FileSystem describes the local filesystem behavior used by the builder.
type FileSystem interface {
	MkdirAll(path string, perm fs.FileMode) error
	IsFile(path string) (bool, error)
	Open(path string) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
}

// ConfigDiskBuilder writes NoCloud cidata images.
type ConfigDiskBuilder interface {
	Build(path string, payload ConfigDiskPayload) error
}

// ConfigDiskPayload contains the NoCloud files written to the cidata image.
type ConfigDiskPayload struct {
	// UserData is the Talos machine config written to user-data.
	UserData []byte
	// MetaData is the NoCloud meta-data YAML.
	MetaData []byte
	// NetworkConfig is the optional NoCloud network-config YAML.
	NetworkConfig []byte
}

type sourceImage struct {
	version     string
	url         string
	filename    string
	schematicID string
	platform    string
	arch        string
}
