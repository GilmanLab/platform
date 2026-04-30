package sopsdecrypt

import (
	"context"
	"fmt"

	"github.com/getsops/sops/v3/decrypt"
)

const (
	yamlFormat = "yaml"
)

// Decrypter decrypts YAML SOPS documents with the stable SOPS Go API.
type Decrypter struct{}

// DecryptYAML decrypts encrypted YAML bytes and returns plaintext YAML bytes.
func (d Decrypter) DecryptYAML(ctx context.Context, encrypted []byte) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cleartext, err := decrypt.Data(encrypted, yamlFormat)
	if err != nil {
		return nil, fmt.Errorf("decrypt SOPS YAML: %w", err)
	}

	return cleartext, nil
}
