package secrets

import "context"

// LocalSource fetches encrypted documents from a local secrets repository.
type LocalSource interface {
	Configured(request Request) bool
	FetchEncrypted(ctx context.Context, request Request) ([]byte, error)
}

// RemoteSource fetches encrypted documents from a remote secrets source.
type RemoteSource interface {
	FetchEncrypted(ctx context.Context, request Request) ([]byte, error)
}

// Decrypter decrypts SOPS-encrypted YAML documents.
type Decrypter interface {
	DecryptYAML(ctx context.Context, encrypted []byte) ([]byte, error)
}

// FieldExtractor extracts a selected field from decrypted YAML.
type FieldExtractor interface {
	ExtractYAML(ctx context.Context, document []byte, pointer string) ([]byte, error)
}
