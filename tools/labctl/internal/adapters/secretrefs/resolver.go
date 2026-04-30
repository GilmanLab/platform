package secretrefs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gilmanlab/platform/tools/labctl/internal/app/incusosimage"
	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

// Service fetches decrypted lab secrets.
type Service interface {
	Get(ctx context.Context, request appsecrets.Request) (appsecrets.Result, error)
}

// Options controls how schema secret references are fetched.
type Options struct {
	// Ref is the Git ref used by remote fetches.
	Ref string
	// Source selects local, GitHub, or automatic source behavior.
	Source appsecrets.SourceMode
	// LocalRepoDir is an explicit local checkout path for the secrets repository.
	LocalRepoDir string
	// BrokerFunction is the AWS Lambda function name used for broker token minting.
	BrokerFunction string
	// AWSRegion is an optional AWS region override for broker invocation.
	AWSRegion string
}

// Resolver resolves IncusOS seed secret references through the shared secrets service.
type Resolver struct {
	service Service
	options Options
}

// NewResolver constructs a Resolver.
func NewResolver(service Service, options Options) Resolver {
	return Resolver{
		service: service,
		options: options,
	}
}

// Resolve returns the plaintext string selected by a schema secret reference.
func (r Resolver) Resolve(ctx context.Context, ref incusosimage.SecretRef) (string, error) {
	if r.service == nil {
		return "", errors.New("secrets service is required")
	}
	if strings.TrimSpace(ref.Pointer) == "" {
		return "", errors.New("secret pointer is required for IncusOS string secrets")
	}

	result, err := r.service.Get(ctx, appsecrets.Request{
		Path:           ref.Path,
		Ref:            r.options.Ref,
		Source:         r.options.Source,
		LocalRepoDir:   r.options.LocalRepoDir,
		Field:          ref.Pointer,
		BrokerFunction: r.options.BrokerFunction,
		AWSRegion:      r.options.AWSRegion,
	})
	if err != nil {
		return "", fmt.Errorf("get secret %s%s: %w", ref.Path, ref.Pointer, err)
	}

	return string(result.Data), nil
}
