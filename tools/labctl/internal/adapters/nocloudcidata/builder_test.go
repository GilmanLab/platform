package nocloudcidata_test

import (
	"io/fs"
	"strings"
	"testing"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/nocloudcidata"
	"github.com/gilmanlab/platform/tools/labctl/internal/app/talosimage"
)

func TestBuilderWritesNoCloudCIDATAImage(t *testing.T) {
	path := t.TempDir() + "/cidata.img"
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

func readFile(t *testing.T, fsys fs.ReadFileFS, name string) []byte {
	t.Helper()

	data, err := fsys.ReadFile(name)
	require.NoError(t, err)

	return data
}
