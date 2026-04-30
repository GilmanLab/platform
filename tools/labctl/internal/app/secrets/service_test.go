package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanPath(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError string
	}{
		{
			name:  "normalizes clean relative path",
			input: "network//vyos/router.sops.yaml",
			want:  "network/vyos/router.sops.yaml",
		},
		{
			name:  "strips leading dot segment",
			input: "./network/vyos/router.sops.yaml",
			want:  "network/vyos/router.sops.yaml",
		},
		{name: "rejects empty path", input: "", wantError: "secret path is required"},
		{
			name:      "rejects absolute path",
			input:     "/network/vyos/router.sops.yaml",
			wantError: "must be repository-relative",
		},
		{name: "rejects parent segment", input: "network/../router.sops.yaml", wantError: "must not contain '..'"},
		{name: "rejects backslash", input: `network\vyos.yaml`, wantError: "must use forward slashes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CleanPath(tt.input)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServiceGetSelectsSource(t *testing.T) {
	tests := []struct {
		name            string
		request         Request
		localConfigured bool
		wantLocalCalls  int
		wantRemoteCalls int
	}{
		{
			name:            "auto uses configured local source",
			request:         Request{Path: "secret.sops.yaml", Source: SourceAuto},
			localConfigured: true,
			wantLocalCalls:  1,
		},
		{
			name:            "auto uses remote source when local is not configured",
			request:         Request{Path: "secret.sops.yaml", Source: SourceAuto},
			wantRemoteCalls: 1,
		},
		{
			name:            "explicit github ignores configured local source",
			request:         Request{Path: "secret.sops.yaml", Source: SourceGitHub},
			localConfigured: true,
			wantRemoteCalls: 1,
		},
		{
			name:            "explicit local uses local source",
			request:         Request{Path: "secret.sops.yaml", Source: SourceLocal},
			localConfigured: true,
			wantLocalCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := &fakeLocalSource{configured: tt.localConfigured, data: []byte("encrypted")}
			remote := &fakeRemoteSource{data: []byte("encrypted")}
			decrypter := &fakeDecrypter{data: []byte("decrypted: true\n")}
			extractor := &fakeExtractor{data: []byte("selected\n")}
			service := NewService(Dependencies{
				Local:          local,
				Remote:         remote,
				Decrypter:      decrypter,
				FieldExtractor: extractor,
			})

			result, err := service.Get(context.Background(), tt.request)

			require.NoError(t, err)
			assert.Equal(t, []byte("decrypted: true\n"), result.Data)
			assert.Equal(t, tt.wantLocalCalls, local.calls)
			assert.Equal(t, tt.wantRemoteCalls, remote.calls)
			assert.Equal(t, 1, decrypter.calls)
			assert.Zero(t, extractor.calls)
		})
	}
}

func TestServiceGetExtractsRequestedField(t *testing.T) {
	local := &fakeLocalSource{configured: true, data: []byte("encrypted")}
	extractor := &fakeExtractor{data: []byte("value\n")}
	service := NewService(Dependencies{
		Local:          local,
		Remote:         &fakeRemoteSource{},
		Decrypter:      &fakeDecrypter{data: []byte("field: value\n")},
		FieldExtractor: extractor,
	})

	result, err := service.Get(context.Background(), Request{
		Path:   "secret.sops.yaml",
		Field:  "/field",
		Source: SourceAuto,
	})

	require.NoError(t, err)
	assert.Equal(t, []byte("value\n"), result.Data)
	assert.Equal(t, "/field", extractor.pointer)
}

func TestServiceGetDoesNotFallbackAfterLocalFailure(t *testing.T) {
	localErr := errors.New("local failure")
	local := &fakeLocalSource{configured: true, err: localErr}
	remote := &fakeRemoteSource{data: []byte("encrypted")}
	service := NewService(Dependencies{
		Local:          local,
		Remote:         remote,
		Decrypter:      &fakeDecrypter{data: []byte("decrypted: true\n")},
		FieldExtractor: &fakeExtractor{},
	})

	_, err := service.Get(context.Background(), Request{Path: "secret.sops.yaml", Source: SourceAuto})

	require.ErrorIs(t, err, localErr)
	assert.Equal(t, 1, local.calls)
	assert.Zero(t, remote.calls)
}

func TestServiceGetPropagatesValidationAndFieldErrors(t *testing.T) {
	tests := []struct {
		name      string
		request   Request
		extractor *fakeExtractor
		wantError string
	}{
		{
			name:      "invalid path",
			request:   Request{Path: "../secret.sops.yaml"},
			extractor: &fakeExtractor{},
			wantError: "must not contain '..'",
		},
		{
			name:      "invalid source",
			request:   Request{Path: "secret.sops.yaml", Source: "bad"},
			extractor: &fakeExtractor{},
			wantError: "invalid secrets source",
		},
		{
			name:      "field extractor failure",
			request:   Request{Path: "secret.sops.yaml", Source: SourceGitHub, Field: "/missing"},
			extractor: &fakeExtractor{err: errors.New("missing field")},
			wantError: "extract /missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(Dependencies{
				Local:          &fakeLocalSource{},
				Remote:         &fakeRemoteSource{data: []byte("encrypted")},
				Decrypter:      &fakeDecrypter{data: []byte("decrypted: true\n")},
				FieldExtractor: tt.extractor,
			})

			_, err := service.Get(context.Background(), tt.request)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

type fakeLocalSource struct {
	configured bool
	data       []byte
	err        error
	calls      int
}

func (s *fakeLocalSource) Configured(Request) bool {
	return s.configured
}

func (s *fakeLocalSource) FetchEncrypted(context.Context, Request) ([]byte, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}

	return s.data, nil
}

type fakeRemoteSource struct {
	data  []byte
	err   error
	calls int
}

func (s *fakeRemoteSource) FetchEncrypted(context.Context, Request) ([]byte, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}

	return s.data, nil
}

type fakeDecrypter struct {
	data  []byte
	err   error
	calls int
}

func (d *fakeDecrypter) DecryptYAML(context.Context, []byte) ([]byte, error) {
	d.calls++
	if d.err != nil {
		return nil, d.err
	}

	return d.data, nil
}

type fakeExtractor struct {
	data    []byte
	err     error
	calls   int
	pointer string
}

func (e *fakeExtractor) ExtractYAML(_ context.Context, _ []byte, pointer string) ([]byte, error) {
	e.calls++
	e.pointer = pointer
	if e.err != nil {
		return nil, e.err
	}

	return e.data, nil
}
