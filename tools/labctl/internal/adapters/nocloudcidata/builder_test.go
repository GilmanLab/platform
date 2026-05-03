package nocloudcidata_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/nocloudcidata"
	"github.com/gilmanlab/platform/tools/labctl/internal/app/talosimage"
)

func TestBuilderWritesNoCloudCIDATAImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cidata.img")
	payload := talosimage.ConfigDiskPayload{
		UserData:      []byte("machine:\n  type: controlplane\n"),
		MetaData:      []byte("instance-id: test\nlocal-hostname: bootstrap\n"),
		NetworkConfig: []byte("version: 1\n"),
	}

	err := nocloudcidata.New().Build(path, payload)

	require.NoError(t, err)
	diskImage, err := diskfs.Open(path, diskfs.WithOpenMode(diskfs.ReadOnly))
	require.NoError(t, err)
	cidata, err := diskImage.GetFilesystem(0)
	require.NoError(t, err)
	defer cidata.Close()

	assert.Equal(t, "CIDATA", strings.TrimSpace(cidata.Label()))
	assert.Equal(t, payload.UserData, readFile(t, cidata, "/user-data"))
	assert.Equal(t, payload.MetaData, readFile(t, cidata, "/meta-data"))
	assert.Equal(t, payload.NetworkConfig, readFile(t, cidata, "/network-config"))
}

func TestBuilderSetsImageMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cidata.img")

	err := nocloudcidata.New().Build(path, talosimage.ConfigDiskPayload{
		UserData: []byte("machine:\n  type: controlplane\n"),
		MetaData: []byte("instance-id: test\nlocal-hostname: bootstrap\n"),
	})

	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm(), "cidata image must be 0600")
}

func TestBuilderHandlesPayloadLargerThanDefaultFloor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cidata.img")
	bigUserData := bytes.Repeat([]byte("a"), 32*1024*1024)

	err := nocloudcidata.New().Build(path, talosimage.ConfigDiskPayload{
		UserData: bigUserData,
		MetaData: []byte("instance-id: test\nlocal-hostname: bootstrap\n"),
	})

	require.NoError(t, err)

	diskImage, err := diskfs.Open(path, diskfs.WithOpenMode(diskfs.ReadOnly))
	require.NoError(t, err)
	cidata, err := diskImage.GetFilesystem(0)
	require.NoError(t, err)
	defer cidata.Close()

	got := readFile(t, cidata, "/user-data")
	assert.Equal(t, bigUserData, got)
}

func TestBuilderRejectsPayloadOverMaxSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cidata.img")
	overflow := bytes.Repeat([]byte("a"), int(nocloudcidata.CidataMaxSize)+1)

	err := nocloudcidata.New().Build(path, talosimage.ConfigDiskPayload{
		UserData: overflow,
		MetaData: []byte("instance-id: test\nlocal-hostname: bootstrap\n"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds the maximum cidata image size")

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "no partial image must be left on disk")
}

func readFile(t *testing.T, fsys fs.ReadFileFS, name string) []byte {
	t.Helper()

	data, err := fsys.ReadFile(name)
	require.NoError(t, err)

	return data
}
