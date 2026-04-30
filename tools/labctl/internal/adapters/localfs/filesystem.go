package localfs

import (
	"io"
	"io/fs"
	"os"

	"github.com/gilmanlab/platform/tools/labctl/internal/app/incusosimage"
)

// FileSystem reads and writes build artifacts on the local filesystem.
type FileSystem struct{}

// New constructs a local filesystem adapter.
func New() FileSystem {
	return FileSystem{}
}

// MkdirAll creates a directory and any missing parents.
func (FileSystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// IsFile reports whether path exists and is a regular file.
func (FileSystem) IsFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return info.Mode().IsRegular(), nil
}

// Open opens path for reading.
func (FileSystem) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

// Create creates or truncates path for writing.
func (FileSystem) Create(path string) (io.WriteCloser, error) {
	return os.Create(path)
}

// OpenReadWrite opens path for in-place mutation.
func (FileSystem) OpenReadWrite(path string) (incusosimage.WritableFile, error) {
	return os.OpenFile(path, os.O_RDWR, 0)
}
