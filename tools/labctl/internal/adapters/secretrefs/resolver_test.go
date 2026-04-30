package secretrefs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/secretrefs"
	"github.com/gilmanlab/platform/tools/labctl/internal/app/incusosimage"
	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

func TestResolverMapsSecretRefToSharedSecretsRequest(t *testing.T) {
	service := &fakeSecretsService{
		result: appsecrets.Result{Data: []byte("cert\n")},
	}
	resolver := secretrefs.NewResolver(service, secretrefs.Options{
		Ref:            "feature",
		Source:         appsecrets.SourceGitHub,
		LocalRepoDir:   "/tmp/secrets",
		BrokerFunction: "broker",
		AWSRegion:      "us-west-2",
	})

	got, err := resolver.Resolve(context.Background(), incusosimage.SecretRef{
		Path:    "compute/incusos/bootstrap-client.sops.yaml",
		Pointer: "/client_crt_pem",
	})

	require.NoError(t, err)
	assert.Equal(t, "cert\n", got)
	assert.Equal(t, appsecrets.Request{
		Path:           "compute/incusos/bootstrap-client.sops.yaml",
		Ref:            "feature",
		Source:         appsecrets.SourceGitHub,
		LocalRepoDir:   "/tmp/secrets",
		Field:          "/client_crt_pem",
		BrokerFunction: "broker",
		AWSRegion:      "us-west-2",
	}, service.request)
}

func TestResolverRequiresPointerForStringSecret(t *testing.T) {
	resolver := secretrefs.NewResolver(&fakeSecretsService{}, secretrefs.Options{})

	_, err := resolver.Resolve(context.Background(), incusosimage.SecretRef{
		Path: "compute/incusos/bootstrap-client.sops.yaml",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret pointer is required")
}

type fakeSecretsService struct {
	request appsecrets.Request
	result  appsecrets.Result
	err     error
}

func (f *fakeSecretsService) Get(_ context.Context, request appsecrets.Request) (appsecrets.Result, error) {
	f.request = request

	return f.result, f.err
}
