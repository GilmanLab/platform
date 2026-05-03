package nocloudcidata

import (
	"fmt"
	"os"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"

	"github.com/gilmanlab/platform/tools/labctl/internal/app/talosimage"
)

const (
	cidataSizeBytes = 16 * 1024 * 1024
	volumeLabel     = "CIDATA"
)

// Builder writes NoCloud cidata disk images.
type Builder struct{}

// New constructs a Builder.
func New() Builder {
	return Builder{}
}

// Build writes a FAT32 NoCloud cidata image to path.
func (Builder) Build(path string, payload talosimage.ConfigDiskPayload) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing cidata image %q: %w", path, err)
	}

	diskImage, err := diskfs.Create(path, cidataSizeBytes, diskfs.SectorSize512)
	if err != nil {
		return fmt.Errorf("create cidata image %q: %w", path, err)
	}

	cidata, err := diskImage.CreateFilesystem(disk.FilesystemSpec{
		Partition:    0,
		FSType:       filesystem.TypeFat32,
		VolumeLabel:  volumeLabel,
		Reproducible: true,
	})
	if err != nil {
		return fmt.Errorf("create cidata filesystem %q: %w", path, err)
	}
	defer cidata.Close()

	if err := writeFile(cidata, "/user-data", payload.UserData); err != nil {
		return err
	}
	if err := writeFile(cidata, "/meta-data", payload.MetaData); err != nil {
		return err
	}
	if len(payload.NetworkConfig) > 0 {
		if err := writeFile(cidata, "/network-config", payload.NetworkConfig); err != nil {
			return err
		}
	}

	return nil
}

func writeFile(cidata filesystem.FileSystem, path string, data []byte) error {
	file, err := cidata.OpenFile(path, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return fmt.Errorf("create cidata file %q: %w", path, err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write cidata file %q: %w", path, err)
	}

	return nil
}
