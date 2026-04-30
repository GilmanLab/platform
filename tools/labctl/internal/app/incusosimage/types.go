package incusosimage

import (
	"context"
	"io"
	"io/fs"

	schemaincusos "github.com/gilmanlab/platform/schemas/lab/incusos"
)

// Request describes an IncusOS image build request.
type Request struct {
	// Config is the validated IncusOS image build configuration.
	Config schemaincusos.ImageBuild
	// BaseDir resolves relative output paths in Config.
	BaseDir string
	// Secrets resolves secret-backed seed values for this build.
	Secrets SecretResolver
}

// Result describes the built IncusOS image artifact.
type Result struct {
	// Name is the stable build name from the input configuration.
	Name string `json:"name"`
	// ArtifactPath is the local path to the generated compressed image.
	ArtifactPath string `json:"artifactPath"`
	// SourceVersion is the selected upstream IncusOS image version.
	SourceVersion string `json:"sourceVersion"`
	// SourceURL is the selected upstream IncusOS image URL.
	SourceURL string `json:"sourceURL"`
	// SourceSHA256 is the expected SHA256 digest for the upstream image.
	SourceSHA256 string `json:"sourceSHA256"`
}

// Dependencies groups external ports needed to build an IncusOS image.
type Dependencies struct {
	// Upstream reads IncusOS image metadata and downloads upstream artifacts.
	Upstream Upstream
	// Files reads and writes local build artifacts.
	Files FileSystem
}

// Upstream reads IncusOS image metadata and artifact bytes.
type Upstream interface {
	FetchIndex(ctx context.Context, url string) (Index, error)
	Download(ctx context.Context, url string) (io.ReadCloser, error)
}

// SecretRef identifies one secret-backed string value needed by an image seed.
type SecretRef struct {
	// Path is the repository-relative path inside GilmanLab/secrets.
	Path string
	// Pointer is the RFC 6901 JSON Pointer selecting the string value.
	Pointer string
}

// SecretResolver resolves a secret reference into its plaintext value.
type SecretResolver interface {
	Resolve(ctx context.Context, ref SecretRef) (string, error)
}

// FileSystem describes the local filesystem behavior used by the builder.
type FileSystem interface {
	MkdirAll(path string, perm fs.FileMode) error
	IsFile(path string) (bool, error)
	Open(path string) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
	OpenReadWrite(path string) (WritableFile, error)
}

// WritableFile is a file opened for in-place image mutation.
type WritableFile interface {
	io.WriteCloser
	io.Seeker
	Truncate(size int64) error
}

// Index is the upstream IncusOS image index shape used by the builder.
type Index struct {
	// Updates is the ordered set of upstream image updates.
	Updates []Update `json:"updates"`
}

// Update describes one upstream IncusOS image update.
type Update struct {
	// Version identifies the upstream image version.
	Version string `json:"version"`
	// URL is the update-relative artifact path.
	URL string `json:"url"`
	// Channels lists the release channels that include this update.
	Channels []string `json:"channels"`
	// Files lists artifacts attached to this update.
	Files []File `json:"files"`
}

// File describes one upstream IncusOS image artifact.
type File struct {
	// Architecture is the artifact CPU architecture.
	Architecture string `json:"architecture"`
	// Component is the IncusOS artifact component.
	Component string `json:"component"`
	// Type is the IncusOS artifact type.
	Type string `json:"type"`
	// Filename is the artifact file name.
	Filename string `json:"filename"`
	// SHA256 is the artifact SHA256 digest.
	SHA256 string `json:"sha256"`
}

type sourceImage struct {
	version  string
	url      string
	filename string
	sha256   string
}
