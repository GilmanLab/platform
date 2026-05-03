package talosimage_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	schematalos "github.com/gilmanlab/platform/schemas/lab/talos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/localfs"
	"github.com/gilmanlab/platform/tools/labctl/internal/app/talosimage"
)

func TestServiceBuildWritesBootImageAndNoCloudPayload(t *testing.T) {
	baseDir := t.TempDir()
	writeFixture(t, baseDir, "controlplane.yaml", "machine:\n  type: controlplane\n")
	writeFixture(t, baseDir, "network.yaml", "version: 1\n")

	raw := []byte("talos-raw-image")
	upstream := &fakeUpstream{artifact: xzBytes(t, raw)}
	configDisk := &fakeConfigDiskBuilder{}
	service := talosimage.NewService(talosimage.Dependencies{
		Upstream:   upstream,
		Files:      localfs.New(),
		ConfigDisk: configDisk,
	})

	result, err := service.Build(context.Background(), talosimage.Request{
		Config:  testConfig(),
		BaseDir: baseDir,
	})

	require.NoError(t, err)
	assert.Equal(t, "talos-test", result.Name)
	assert.Equal(
		t,
		"https://factory.example.test/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/v1.13.0/nocloud-amd64.raw.xz",
		result.SourceURL,
	)
	assert.Equal(t, "v1.13.0", result.SourceVersion)
	assert.Equal(t, "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba", result.SourceSchematicID)
	assert.Equal(t, "nocloud", result.Platform)
	assert.Equal(t, "amd64", result.Arch)
	assert.Equal(t, "img", result.Format)
	assert.Equal(t, result.SourceURL, upstream.url)

	boot, err := os.ReadFile(filepath.Clean(result.BootArtifactPath))
	require.NoError(t, err)
	assert.Equal(t, raw, boot)

	require.Equal(t, result.ConfigArtifactPath, configDisk.path)
	assert.Equal(t, []byte("machine:\n  type: controlplane\n"), configDisk.payload.UserData)
	assert.Contains(t, string(configDisk.payload.MetaData), "instance-id: bootstrap-instance-1")
	assert.Contains(t, string(configDisk.payload.MetaData), "local-hostname: bootstrap-controlplane-1")
	assert.Equal(t, []byte("version: 1\n"), configDisk.payload.NetworkConfig)

	cachePath := filepath.Join(
		baseDir,
		".state",
		"downloads",
		"talos",
		"376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
		"v1.13.0",
		"nocloud-amd64.raw.xz",
	)
	assert.FileExists(t, cachePath)
	assert.Equal(t, 1, upstream.downloads)

	_, err = service.Build(context.Background(), talosimage.Request{
		Config:  testConfig(),
		BaseDir: baseDir,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, upstream.downloads, "expected second build to reuse the cached archive")
}

func testConfig() schematalos.ImageBuild {
	return schematalos.ImageBuild{
		Name: "talos-test",
		Source: schematalos.ImageSource{
			FactoryURL:  "https://factory.example.test",
			Version:     "v1.13.0",
			SchematicID: "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
			Platform:    "nocloud",
			Arch:        "amd64",
			Artifact:    "raw.xz",
		},
		Config: schematalos.MachineConfig{
			Delivery: "nocloud-cidata",
			UserData: schematalos.FileInput{
				Path: "controlplane.yaml",
			},
			MetaData: schematalos.NoCloudMetaData{
				LocalHostname: "bootstrap-controlplane-1",
				InstanceID:    "bootstrap-instance-1",
			},
			NetworkConfig: schematalos.FileInput{
				Path: "network.yaml",
			},
		},
		Output: schematalos.ImageOutput{
			Dir:                ".state/images",
			Format:             "img",
			BootArtifactName:   "talos-boot.img",
			ConfigArtifactName: "talos-cidata.img",
		},
	}
}

type fakeUpstream struct {
	artifact  []byte
	downloads int
	url       string
}

func (f *fakeUpstream) Download(_ context.Context, url string) (io.ReadCloser, error) {
	f.downloads++
	f.url = url

	return io.NopCloser(bytes.NewReader(f.artifact)), nil
}

type fakeConfigDiskBuilder struct {
	path    string
	payload talosimage.ConfigDiskPayload
}

func (f *fakeConfigDiskBuilder) Build(path string, payload talosimage.ConfigDiskPayload) error {
	f.path = path
	f.payload = payload

	return nil
}

func writeFixture(t *testing.T, dir string, name string, data string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600))
}

func xzBytes(t *testing.T, data []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer, err := xz.NewWriter(&buffer)
	require.NoError(t, err)
	_, err = writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return buffer.Bytes()
}
