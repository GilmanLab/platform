package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSecretData(t *testing.T) {
	t.Parallel()

	yamlData := []byte(
		"database:\n  username: admin\n  password: test-secret\nservers:\n  - name: keycloak\n    port: 8080\n",
	)

	got, err := renderSecretData(secretOutputFormatYAML, yamlData)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(yamlData, got))

	got, err = renderSecretData(secretOutputFormatJSON, yamlData)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"database": {
			"username": "admin",
			"password": "test-secret"
		},
		"servers": [
			{
				"name": "keycloak",
				"port": 8080
			}
		]
	}`, string(got))
}

func TestRenderSecretDataJSONFieldValues(t *testing.T) {
	t.Parallel()

	structured, err := renderSecretData(secretOutputFormatJSON, []byte("name: keycloak\nport: 8080\n"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"keycloak","port":8080}`, string(structured))

	scalar, err := renderSecretData(secretOutputFormatJSON, []byte("admin\n"))
	require.NoError(t, err)
	assert.Equal(t, "\"admin\"\n", string(scalar))
}

func TestRenderSecretDataRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	_, err := renderSecretData("toml", []byte("username: admin\n"))
	require.ErrorContains(t, err, `invalid output format "toml"`)
}

func TestWriteSecretDataStdout(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := writeSecretData(&out, "-", []byte("username: admin\n"))
	require.NoError(t, err)
	assert.Equal(t, "username: admin\n", out.String())
}

func TestWritePrivateOutputFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "secret.yaml")

	require.NoError(t, writeSecretData(&bytes.Buffer{}, path, []byte("first\n")))
	assertFileMode(t, path, privateOutputFileMode)
	assertFileContents(t, path, "first\n")

	require.NoError(t, writeSecretData(&bytes.Buffer{}, path, []byte("second\n")))
	assertFileMode(t, path, privateOutputFileMode)
	assertFileContents(t, path, "second\n")
}

func TestWritePrivateOutputFileRequiresExistingParent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "secret.yaml")

	err := writeSecretData(&bytes.Buffer{}, path, []byte("username: admin\n"))
	require.ErrorContains(t, err, "output parent directory")
	require.NoFileExists(t, path)
}

func assertFileMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, expected, info.Mode().Perm())
}

func assertFileContents(t *testing.T, path string, expected string) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(data))
}
