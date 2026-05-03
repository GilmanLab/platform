package talosimage_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	assert.Equal(t, result.SourceURL, upstream.downloadURL)
	assert.Equal(t, result.SourceURL+".sha256", upstream.checksumURL)

	boot, err := os.ReadFile(filepath.Clean(result.BootArtifactPath))
	require.NoError(t, err)
	assert.Equal(t, raw, boot)
	assert.Equal(t, hashHex(raw), result.BootArtifactSHA256)

	require.Equal(t, result.ConfigArtifactPath, configDisk.path)
	assert.Equal(t, []byte("machine:\n  type: controlplane\n"), configDisk.payload.UserData)
	assert.Contains(t, string(configDisk.payload.MetaData), "instance-id: bootstrap-instance-1")
	assert.Contains(t, string(configDisk.payload.MetaData), "local-hostname: bootstrap-controlplane-1")
	assert.Equal(t, []byte("version: 1\n"), configDisk.payload.NetworkConfig)

	cidata, err := os.ReadFile(filepath.Clean(result.ConfigArtifactPath))
	require.NoError(t, err)
	assert.Equal(t, hashHex(cidata), result.ConfigArtifactSHA256)

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

func TestServiceBuildRedownloadsCorruptCache(t *testing.T) {
	baseDir := t.TempDir()
	writeFixture(t, baseDir, "controlplane.yaml", "machine:\n  type: controlplane\n")

	raw := []byte("talos-raw-image")
	upstream := &fakeUpstream{artifact: xzBytes(t, raw)}
	configDisk := &fakeConfigDiskBuilder{}
	service := talosimage.NewService(talosimage.Dependencies{
		Upstream:   upstream,
		Files:      localfs.New(),
		ConfigDisk: configDisk,
	})

	cachePath := filepath.Join(
		baseDir,
		".state",
		"downloads",
		"talos",
		"376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
		"v1.13.0",
		"nocloud-amd64.raw.xz",
	)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.WriteFile(cachePath, []byte("corrupt-cache-content"), 0o600))

	cfg := testConfig()
	cfg.Config.NetworkConfig = schematalos.FileInput{}
	_, err := service.Build(context.Background(), talosimage.Request{
		Config:  cfg,
		BaseDir: baseDir,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, upstream.downloads, "expected corrupt cache to trigger a redownload")

	cached, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	assert.Equal(t, upstream.artifact, cached)
}

func TestServiceBuildRejectsSidecarMismatch(t *testing.T) {
	baseDir := t.TempDir()
	writeFixture(t, baseDir, "controlplane.yaml", "machine:\n  type: controlplane\n")

	raw := []byte("talos-raw-image")
	upstream := &fakeUpstream{
		artifact: xzBytes(t, raw),
		// Force the sidecar response to a digest that does not match what
		// the service will compute on the streamed body.
		sha256Override: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	configDisk := &fakeConfigDiskBuilder{}
	service := talosimage.NewService(talosimage.Dependencies{
		Upstream:   upstream,
		Files:      localfs.New(),
		ConfigDisk: configDisk,
	})

	cfg := testConfig()
	cfg.Config.NetworkConfig = schematalos.FileInput{}
	_, err := service.Build(context.Background(), talosimage.Request{
		Config:  cfg,
		BaseDir: baseDir,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sha256 mismatch")

	downloadsDir := filepath.Join(
		baseDir,
		".state",
		"downloads",
		"talos",
		"376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
		"v1.13.0",
	)
	entries, err := os.ReadDir(downloadsDir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".tmp-", "temporary archive must not be left behind")
	}
}

func TestServiceBuildPropagatesSidecarError(t *testing.T) {
	baseDir := t.TempDir()
	writeFixture(t, baseDir, "controlplane.yaml", "machine:\n  type: controlplane\n")

	upstream := &fakeUpstream{
		artifact:    xzBytes(t, []byte("talos-raw-image")),
		sha256Error: errors.New("sidecar unreachable"),
	}
	configDisk := &fakeConfigDiskBuilder{}
	service := talosimage.NewService(talosimage.Dependencies{
		Upstream:   upstream,
		Files:      localfs.New(),
		ConfigDisk: configDisk,
	})

	cfg := testConfig()
	cfg.Config.NetworkConfig = schematalos.FileInput{}
	_, err := service.Build(context.Background(), talosimage.Request{
		Config:  cfg,
		BaseDir: baseDir,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch Talos image checksum")
	assert.Equal(t, 0, upstream.downloads, "no download must happen when the sidecar fetch fails")
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
	artifact       []byte
	sha256Override string
	sha256Error    error
	downloads      int
	downloadURL    string
	checksumURL    string
}

func (f *fakeUpstream) Download(_ context.Context, url string) (io.ReadCloser, error) {
	f.downloads++
	f.downloadURL = url

	return io.NopCloser(bytes.NewReader(f.artifact)), nil
}

func (f *fakeUpstream) FetchSHA256(_ context.Context, url string) (string, error) {
	f.checksumURL = url
	if f.sha256Error != nil {
		return "", f.sha256Error
	}
	if f.sha256Override != "" {
		return f.sha256Override, nil
	}

	return hashHex(f.artifact), nil
}

type fakeConfigDiskBuilder struct {
	path    string
	payload talosimage.ConfigDiskPayload
}

func (f *fakeConfigDiskBuilder) Build(path string, payload talosimage.ConfigDiskPayload) error {
	f.path = path
	f.payload = payload

	// Write a placeholder file so the service can hash the artifact.
	return os.WriteFile(path, payload.UserData, 0o600)
}

func writeFixture(t *testing.T, dir, name, data string) {
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

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}
