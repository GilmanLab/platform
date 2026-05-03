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
	mibibyte = 1024 * 1024
	// cidataMinSize is the floor used by Builder. FAT32 needs at least this
	// many bytes to format reliably with the diskfs defaults.
	cidataMinSize int64 = 16 * mibibyte
	// CidataMaxSize bounds the cidata image. A Talos machine config larger
	// than the resulting payload limit is a strong signal that the operator
	// is embedding something that should live in a side channel rather than
	// on a NoCloud cidata disk.
	CidataMaxSize int64 = 64 * mibibyte
	// cidataOverhead is a generous reserve for FAT32 tables, reserved
	// sectors, and cluster alignment so that small payloads always fit.
	cidataOverhead int64 = 4 * mibibyte

	volumeLabel     = "CIDATA"
	cidataImageMode = 0o600
)

// Builder writes NoCloud cidata disk images.
type Builder struct{}

// New constructs a Builder.
func New() Builder {
	return Builder{}
}

// Build writes a FAT32 NoCloud cidata image to path.
//
// The image is sized to fit the payload with FAT32 overhead, capped at
// [CidataMaxSize]; payloads that would require a larger image surface a
// clear error rather than the diskfs library's lower-level failure. The
// output file is set to mode 0600 because Talos machine configuration
// embedded in user-data is secret material — it carries cluster PKI and
// API credentials and must be protected accordingly.
func (Builder) Build(path string, payload talosimage.ConfigDiskPayload) error {
	size, err := sizeFor(payload)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing cidata image %q: %w", path, err)
	}

	diskImage, err := diskfs.Create(path, size, diskfs.SectorSize512)
	if err != nil {
		return fmt.Errorf("create cidata image %q: %w", path, err)
	}

	err = os.Chmod(path, cidataImageMode)
	if err != nil {
		return fmt.Errorf("set cidata image %q mode: %w", path, err)
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

	err = writeFile(cidata, "/user-data", payload.UserData)
	if err != nil {
		return err
	}
	err = writeFile(cidata, "/meta-data", payload.MetaData)
	if err != nil {
		return err
	}
	if len(payload.NetworkConfig) > 0 {
		err = writeFile(cidata, "/network-config", payload.NetworkConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func sizeFor(payload talosimage.ConfigDiskPayload) (int64, error) {
	payloadSize := int64(len(payload.UserData) + len(payload.MetaData) + len(payload.NetworkConfig))

	want := roundUpMiB(max(payloadSize+cidataOverhead, cidataMinSize))

	if want > CidataMaxSize {
		return 0, fmt.Errorf(
			"cidata payload (%d bytes) requires %d bytes which exceeds the maximum cidata image size of %d bytes",
			payloadSize,
			want,
			CidataMaxSize,
		)
	}

	return want, nil
}

func roundUpMiB(n int64) int64 {
	return ((n + mibibyte - 1) / mibibyte) * mibibyte
}

func writeFile(cidata filesystem.FileSystem, path string, data []byte) error {
	file, err := cidata.OpenFile(path, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return fmt.Errorf("create cidata file %q: %w", path, err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("write cidata file %q: %w", path, err)
	}

	return nil
}
