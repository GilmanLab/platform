package yamldoc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractorExtractYAML(t *testing.T) {
	document := []byte(`
database:
  username: admin
  ports:
    - 5432
    - 6432
escaped:
  literal/slash: value
  "~tilde": marker
items:
  - name: keycloak
    enabled: true
`)

	tests := []struct {
		name      string
		pointer   string
		want      string
		wantError string
	}{
		{name: "scalar field", pointer: "/database/username", want: "admin\n"},
		{name: "array item", pointer: "/database/ports/1", want: "6432\n"},
		{name: "structured field", pointer: "/items/0", want: "name: keycloak\nenabled: true\n"},
		{name: "escaped slash", pointer: "/escaped/literal~1slash", want: "value\n"},
		{name: "escaped tilde", pointer: "/escaped/~0tilde", want: "marker\n"},
		{name: "missing field", pointer: "/database/password", wantError: "was not found"},
		{name: "invalid pointer", pointer: "database/username", wantError: "start with '/'"},
		{name: "invalid escape", pointer: "/escaped/~2", wantError: "invalid JSON Pointer escape"},
	}

	extractor := Extractor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractor.ExtractYAML(context.Background(), document, tt.pointer)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}
