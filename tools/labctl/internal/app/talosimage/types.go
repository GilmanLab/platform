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
	// BootArtifactSHA256 is the lowercase hex SHA256 digest of the boot disk image.
	BootArtifactSHA256 string `json:"bootArtifactSHA256"`
	// ConfigArtifactPath is the local path to the NoCloud cidata image.
	ConfigArtifactPath string `json:"configArtifactPath"`
	// ConfigArtifactSHA256 is the lowercase hex SHA256 digest of the NoCloud cidata image.
	ConfigArtifactSHA256 string `json:"configArtifactSHA256"`
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
	// Upstream downloads Talos Image Factory artifacts and their digests.
	Upstream Upstream
	// Files reads and writes local build artifacts.
	Files FileSystem
	// ConfigDisk builds NoCloud cidata images.
	ConfigDisk ConfigDiskBuilder
}

// Upstream downloads Talos Image Factory artifact bytes and digests.
type Upstream interface {
	// Download opens a streaming response body for the artifact at url.
	Download(ctx context.Context, url string) (io.ReadCloser, error)
	// FetchSHA256 returns the lowercase hex SHA256 digest published at url.
	FetchSHA256(ctx context.Context, url string) (string, error)
}

// FileSystem describes the local filesystem behavior used by the builder.
type FileSystem interface {
	// MkdirAll creates a directory and any missing parents.
	MkdirAll(path string, perm fs.FileMode) error
	// IsFile reports whether path exists and is a regular file.
	IsFile(path string) (bool, error)
	// Open opens path for reading.
	Open(path string) (io.ReadCloser, error)
	// Create creates or truncates path for writing.
	Create(path string) (io.WriteCloser, error)
	// CreateTemp creates a new uniquely-named temporary file in dir matching
	// the supplied name pattern. Callers must close the returned TempFile.
	CreateTemp(dir, pattern string) (TempFile, error)
	// Remove removes the named file.
	Remove(path string) error
	// Rename atomically replaces newPath with oldPath.
	Rename(oldPath, newPath string) error
}

// TempFile is a filesystem-backed temporary file with a known on-disk path.
type TempFile interface {
	io.WriteCloser
	// Name returns the on-disk path of the temporary file.
	Name() string
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
