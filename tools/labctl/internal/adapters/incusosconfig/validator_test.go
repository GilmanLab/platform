package incusosconfig_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/incusosconfig"
)

func TestValidatorAppliesSchemaDefaults(t *testing.T) {
	config, err := incusosconfig.New().ValidateYAML("valid.yaml", []byte(validConfigYAML()))

	require.NoError(t, err)
	assert.Equal(t, "stable", config.Source.Channel)
	assert.Equal(t, "x86_64", config.Source.Arch)
	assert.Equal(t, "latest", string(config.Source.Version))
	assert.Equal(t, "img.gz", string(config.Output.Format))
	assert.Equal(t, "1", config.Seed.Applications.Version)
	assert.Equal(t, "1", config.Seed.Incus.Version)
	assert.True(t, config.Seed.Incus.ApplyDefaults)
	assert.Equal(t, "client", config.Seed.Incus.Preseed.Certificates[0].Type)
}

func TestValidatorRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		message string
	}{
		{
			name: "unknown field",
			input: validConfigYAML() + `
unexpected: true
`,
			message: "field not allowed",
		},
		{
			name: "bad format",
			input: replace(validConfigYAML(), "artifactName: incusos.img.gz", `artifactName: incusos.iso
  format: iso`),
			message: "format must be img.gz",
		},
		{
			name:    "missing required field",
			input:   replace(validConfigYAML(), "name: incusos-test\n", ""),
			message: "field is required but not present",
		},
		{
			name:    "secret string missing pointer",
			input:   replace(validConfigYAML(), "              pointer: /client_crt_pem\n", ""),
			message: "field is required but not present",
		},
		{
			name:    "invalid secret pointer",
			input:   replace(validConfigYAML(), "pointer: /client_crt_pem", "pointer: client_crt_pem"),
			message: "secret string pointer must be an RFC 6901 JSON Pointer",
		},
		{
			name: "absolute secret path",
			input: replace(
				validConfigYAML(),
				"path: compute/incusos/bootstrap-client.sops.yaml",
				"path: /compute/incusos/bootstrap-client.sops.yaml",
			),
			message: "secret path must be repository-relative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := incusosconfig.New().ValidateYAML("invalid.yaml", []byte(tt.input))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func validConfigYAML() string {
	return `name: incusos-test
source:
  indexURL: https://images.example.test/os/index.json
  baseURL: https://images.example.test/os
output:
  dir: .state/images
  artifactName: incusos.img.gz
  size: 1G
seed:
  offset: 4096
  applications:
    applications:
      - name: incus
  incus:
    preseed:
      certificates:
        - name: bootstrap-client
          certificate:
            secretRef:
              path: compute/incusos/bootstrap-client.sops.yaml
              pointer: /client_crt_pem
`
}

func replace(value string, old string, next string) string {
	t := strings.NewReplacer(old, next)

	return t.Replace(value)
}
