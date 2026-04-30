package incusosimage_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	schemaincusos "github.com/gilmanlab/platform/schemas/lab/incusos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/localfs"
	"github.com/gilmanlab/platform/tools/labctl/internal/app/incusosimage"
)

func TestServiceBuildWritesSeededArtifact(t *testing.T) {
	raw := []byte("raw-image")
	archive := gzipBytes(t, raw)
	upstream := &fakeUpstream{
		index:    testIndex(sha256Hex(archive)),
		artifact: archive,
	}
	secrets := fakeSecrets{
		"compute/incusos/bootstrap-client.sops.yaml:/client_crt_pem": "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n",
	}
	service := incusosimage.NewService(incusosimage.Dependencies{
		Upstream: upstream,
		Files:    localfs.New(),
	})

	result, err := service.Build(context.Background(), incusosimage.Request{
		Config:  testConfig(),
		BaseDir: t.TempDir(),
		Secrets: secrets,
	})

	require.NoError(t, err)
	assert.Equal(t, "202604261712", result.SourceVersion)
	assert.Equal(t, "https://images.example.test/os/images/202604261712/incusos.img.gz", result.SourceURL)
	assert.FileExists(t, result.ArtifactPath)

	seedFiles := readSeedFiles(t, result.ArtifactPath, int64(testSeedOffset))
	assert.Contains(t, string(seedFiles["applications.yaml"]), "name: incus")
	assert.Contains(t, string(seedFiles["incus.yaml"]), "name: bootstrap-client")
	assert.Contains(t, string(seedFiles["incus.yaml"]), "-----BEGIN CERTIFICATE-----")
}

func TestServiceBuildRejectsSHA256Mismatch(t *testing.T) {
	service := incusosimage.NewService(incusosimage.Dependencies{
		Upstream: &fakeUpstream{
			index:    testIndex("not-the-right-digest"),
			artifact: gzipBytes(t, []byte("raw-image")),
		},
		Files: localfs.New(),
	})

	_, err := service.Build(context.Background(), incusosimage.Request{
		Config:  testConfig(),
		BaseDir: t.TempDir(),
		Secrets: fakeSecrets{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sha256 mismatch")
}

func TestSelectImageHonorsExplicitVersion(t *testing.T) {
	config := testConfig()
	config.Source.Version = "202604261712"
	archive := gzipBytes(t, []byte("raw-image"))

	service := incusosimage.NewService(incusosimage.Dependencies{
		Upstream: &fakeUpstream{
			index:    testIndex(sha256Hex(archive)),
			artifact: archive,
		},
		Files: localfs.New(),
	})

	result, err := service.Build(context.Background(), incusosimage.Request{
		Config:  config,
		BaseDir: t.TempDir(),
		Secrets: fakeSecrets{
			"compute/incusos/bootstrap-client.sops.yaml:/client_crt_pem": "cert",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "202604261712", result.SourceVersion)
}

const (
	testImageSize  = "8K"
	testSeedOffset = 1024
)

func testConfig() schemaincusos.ImageBuild {
	return schemaincusos.ImageBuild{
		Name: "incusos-test",
		Source: schemaincusos.ImageSource{
			IndexURL: "https://images.example.test/os/index.json",
			BaseURL:  "https://images.example.test/os",
			Channel:  "stable",
			Arch:     "x86_64",
			Version:  "latest",
		},
		Output: schemaincusos.ImageOutput{
			Dir:          ".state/images",
			ArtifactName: "incusos.img.gz",
			Size:         testImageSize,
			Format:       "img.gz",
		},
		Seed: schemaincusos.Seed{
			Offset: testSeedOffset,
			Applications: schemaincusos.ApplicationsSeed{
				Version: "1",
				Applications: []schemaincusos.Application{
					{Name: "incus"},
				},
			},
			Incus: schemaincusos.IncusSeed{
				Version:       "1",
				ApplyDefaults: true,
				Preseed: schemaincusos.IncusPreseed{
					Config: map[string]string{},
					Certificates: []schemaincusos.TrustedClientCertificate{
						{
							Name: "bootstrap-client",
							Type: "client",
							Certificate: schemaincusos.SecretString{
								SecretRef: schemaincusos.SecretStringRef{
									Path:    "compute/incusos/bootstrap-client.sops.yaml",
									Pointer: "/client_crt_pem",
								},
							},
						},
					},
				},
			},
		},
	}
}

func testIndex(digest string) incusosimage.Index {
	return incusosimage.Index{
		Updates: []incusosimage.Update{
			{
				Version:  "202604261712",
				URL:      "/images/202604261712",
				Channels: []string{"stable"},
				Files: []incusosimage.File{
					{
						Architecture: "x86_64",
						Component:    "os",
						Type:         "image-raw",
						Filename:     "incusos.img.gz",
						SHA256:       digest,
					},
				},
			},
		},
	}
}

type fakeUpstream struct {
	index    incusosimage.Index
	artifact []byte
}

func (f *fakeUpstream) FetchIndex(context.Context, string) (incusosimage.Index, error) {
	return f.index, nil
}

func (f *fakeUpstream) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.artifact)), nil
}

type fakeSecrets map[string]string

func (f fakeSecrets) Resolve(_ context.Context, ref incusosimage.SecretRef) (string, error) {
	return f[ref.Path+":"+ref.Pointer], nil
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return buffer.Bytes()
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)

	return hex.EncodeToString(digest[:])
}

func readSeedFiles(t *testing.T, artifactPath string, offset int64) map[string][]byte {
	t.Helper()

	artifact, err := os.Open(filepath.Clean(artifactPath))
	require.NoError(t, err)
	defer artifact.Close()

	gzipReader, err := gzip.NewReader(artifact)
	require.NoError(t, err)
	defer gzipReader.Close()

	raw, err := io.ReadAll(gzipReader)
	require.NoError(t, err)
	require.Greater(t, int64(len(raw)), offset)

	reader := tar.NewReader(bytes.NewReader(raw[offset:]))
	files := map[string][]byte{}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		payload, err := io.ReadAll(reader)
		require.NoError(t, err)
		files[header.Name] = payload
	}

	return files
}
