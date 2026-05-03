package talosconfig_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/talosconfig"
)

func TestValidateYAMLAppliesDefaults(t *testing.T) {
	config, err := talosconfig.New().ValidateYAML("valid.yaml", []byte(validConfigYAML()))

	require.NoError(t, err)
	assert.Equal(t, "talos-test", string(config.Name))
	assert.Equal(t, "https://factory.talos.dev", config.Source.FactoryURL)
	assert.Equal(t, "v1.13.0", string(config.Source.Version))
	assert.Equal(t, "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba", config.Source.SchematicID)
	assert.Equal(t, "nocloud", string(config.Source.Platform))
	assert.Equal(t, "amd64", string(config.Source.Arch))
	assert.Equal(t, "raw.xz", string(config.Source.Artifact))
	assert.Equal(t, "nocloud-cidata", string(config.Config.Delivery))
	assert.Equal(t, "controlplane.yaml", string(config.Config.UserData.Path))
	assert.Equal(t, "bootstrap-controlplane-1", string(config.Config.MetaData.LocalHostname))
	assert.Equal(t, "bootstrap-controlplane-1", config.Config.MetaData.InstanceID)
	assert.Equal(t, "img", string(config.Output.Format))
}

func TestValidateYAMLRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		message string
	}{
		{
			name: "latest version",
			input: replaceValidConfig(
				"version: v1.13.0",
				"version: latest",
			),
			message: "Talos version must be an exact release like v1.13.0",
		},
		{
			name: "unsupported format",
			input: replaceValidConfig(
				"format: img",
				"format: iso",
			),
			message: "format must be img",
		},
		{
			name: "unsupported platform",
			input: replaceValidConfig(
				"version: v1.13.0",
				"version: v1.13.0\n  platform: metal",
			),
			message: "platform must be nocloud",
		},
		{
			name: "parent path",
			input: replaceValidConfig(
				"path: controlplane.yaml",
				"path: ../controlplane.yaml",
			),
			message: "path must be relative and use forward slashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := talosconfig.New().ValidateYAML("invalid.yaml", []byte(tt.input))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func replaceValidConfig(old string, replacement string) string {
	return strings.Replace(validConfigYAML(), old, replacement, 1)
}

func validConfigYAML() string {
	return `name: talos-test
source:
  version: v1.13.0
config:
  userData:
    path: controlplane.yaml
  metaData:
    localHostname: bootstrap-controlplane-1
output:
  dir: .state/images
  format: img
  bootArtifactName: talos-boot.img
  configArtifactName: talos-cidata.img
`
}

func TestValidateYAMLAppliesOutputDefaults(t *testing.T) {
	minimal := `name: talos-test
source:
  version: v1.13.0
config:
  userData:
    path: controlplane.yaml
  metaData:
    localHostname: bootstrap-controlplane-1
`
	config, err := talosconfig.New().ValidateYAML("minimal.yaml", []byte(minimal))

	require.NoError(t, err)
	assert.Equal(t, ".state/images", config.Output.Dir)
	assert.Equal(t, "talos-boot.img", config.Output.BootArtifactName)
	assert.Equal(t, "talos-cidata.img", config.Output.ConfigArtifactName)
	assert.Equal(t, "img", string(config.Output.Format))
}
