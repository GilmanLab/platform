package secretslocal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

const (
	// EnvSecretsDir points at a local checkout of GilmanLab/secrets.
	EnvSecretsDir = "GLAB_SECRETS_DIR" //nolint:gosec // This is an environment variable name, not a secret value.
)

// Source reads encrypted documents from a local secrets repository checkout.
type Source struct {
	lookupEnv func(string) (string, bool)
}

// NewSource constructs a local source.
func NewSource(lookupEnv func(string) (string, bool)) Source {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	return Source{lookupEnv: lookupEnv}
}

// Configured reports whether the request has an explicit or environment-backed local repository.
func (s Source) Configured(request appsecrets.Request) bool {
	_, ok := s.repoDir(request)

	return ok
}

// FetchEncrypted reads an encrypted secret document from the local repository.
func (s Source) FetchEncrypted(ctx context.Context, request appsecrets.Request) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	repoDir, ok := s.repoDir(request)
	if !ok {
		return nil, fmt.Errorf("local secrets repository is not configured; set --repo-dir or %s", EnvSecretsDir)
	}

	filePath := filepath.Join(repoDir, filepath.FromSlash(request.Path))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read local encrypted secret %s: %w", request.Path, err)
	}

	return data, nil
}

func (s Source) repoDir(request appsecrets.Request) (string, bool) {
	if strings.TrimSpace(request.LocalRepoDir) != "" {
		return request.LocalRepoDir, true
	}

	repoDir, ok := s.lookupEnv(EnvSecretsDir)
	if !ok || strings.TrimSpace(repoDir) == "" {
		return "", false
	}

	return repoDir, true
}
