package secrets

import (
	"context"
	"errors"
	"fmt"
)

// Dependencies describes the external ports required by Service.
type Dependencies struct {
	Local          LocalSource
	Remote         RemoteSource
	Decrypter      Decrypter
	FieldExtractor FieldExtractor
}

// Service fetches, decrypts, and optionally filters SOPS-backed secrets.
type Service struct {
	local          LocalSource
	remote         RemoteSource
	decrypter      Decrypter
	fieldExtractor FieldExtractor
}

// NewService constructs a Service from explicit ports.
func NewService(deps Dependencies) Service {
	return Service{
		local:          deps.Local,
		remote:         deps.Remote,
		decrypter:      deps.Decrypter,
		fieldExtractor: deps.FieldExtractor,
	}
}

// Get returns the decrypted secret document or selected field.
func (s Service) Get(ctx context.Context, request Request) (Result, error) {
	cleanPath, err := CleanPath(request.Path)
	if err != nil {
		return Result{}, err
	}
	request.Path = cleanPath
	if request.Ref == "" {
		request.Ref = DefaultRef
	}

	mode, err := normalizeSourceMode(request.Source)
	if err != nil {
		return Result{}, err
	}

	encrypted, err := s.fetchEncrypted(ctx, mode, request)
	if err != nil {
		return Result{}, err
	}

	decrypted, err := s.decrypter.DecryptYAML(ctx, encrypted)
	if err != nil {
		return Result{}, fmt.Errorf("decrypt %s: %w", request.Path, err)
	}

	data := decrypted
	if request.Field != "" {
		data, err = s.fieldExtractor.ExtractYAML(ctx, decrypted, request.Field)
		if err != nil {
			return Result{}, fmt.Errorf("extract %s from %s: %w", request.Field, request.Path, err)
		}
	}

	return Result{
		Path:  request.Path,
		Ref:   request.Ref,
		Field: request.Field,
		Data:  data,
	}, nil
}

func (s Service) fetchEncrypted(ctx context.Context, mode SourceMode, request Request) ([]byte, error) {
	switch mode {
	case SourceAuto:
		if s.local.Configured(request) {
			return s.fetchLocal(ctx, request)
		}

		return s.remote.FetchEncrypted(ctx, request)
	case SourceLocal:
		return s.fetchLocal(ctx, request)
	case SourceGitHub:
		return s.remote.FetchEncrypted(ctx, request)
	default:
		return nil, fmt.Errorf("invalid secrets source %q", mode)
	}
}

func (s Service) fetchLocal(ctx context.Context, request Request) ([]byte, error) {
	if !s.local.Configured(request) {
		return nil, errors.New("local secrets repository is not configured; set --repo-dir or GLAB_SECRETS_DIR")
	}

	return s.local.FetchEncrypted(ctx, request)
}
