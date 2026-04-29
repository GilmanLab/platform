package version_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gilmanlab/platform/tools/labctl/internal/app/version"
)

func TestServiceInfo(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    version.Info
	}{
		{
			name:    "returns injected version",
			version: "v1.2.3",
			want: version.Info{
				Version: "v1.2.3",
			},
		},
		{
			name:    "defaults empty version to dev",
			version: "",
			want: version.Info{
				Version: "dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := version.NewService(tt.version)

			assert.Equal(t, tt.want, service.Info())
		})
	}
}
