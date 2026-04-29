package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/cli"
)

func TestRunVersionUsesInjectedVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run(context.Background(), []string{"version"}, cli.Options{
		Version: "v1.2.3",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	require.Equal(t, 0, code)
	assert.Equal(t, "labctl v1.2.3\n", stdout.String())
	assert.Empty(t, stderr.String())
}

func TestRunVersionSupportsJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run(context.Background(), []string{"version", "--json"}, cli.Options{
		Version: "v1.2.3",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})

	require.Equal(t, 0, code)
	assert.JSONEq(t, `{"version":"v1.2.3"}`, stdout.String())
	assert.Empty(t, stderr.String())
}
