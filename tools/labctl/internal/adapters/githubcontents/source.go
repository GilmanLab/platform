package githubcontents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

const (
	// DefaultBaseURL is the GitHub REST API endpoint.
	DefaultBaseURL = "https://api.github.com"
	// EnvAPIBaseURL overrides GitHub's API endpoint for tests or GitHub Enterprise.
	EnvAPIBaseURL = "LABCTL_GITHUB_API_BASE_URL"

	acceptRawHeader  = "application/vnd.github.raw"
	apiVersionHeader = "X-Github-Api-Version"
	apiVersion       = "2022-11-28"
	repoOwner        = "GilmanLab"
	repoName         = "secrets"
	userAgent        = "labctl"
	maxErrorBytes    = 4096
)

// TokenProvider returns GitHub bearer tokens.
type TokenProvider interface {
	Token(ctx context.Context, request appsecrets.Request) (string, error)
}

// Source fetches encrypted documents through GitHub's Contents API.
type Source struct {
	httpClient    *http.Client
	tokenProvider TokenProvider
	baseURL       string
}

// NewSource constructs a GitHub Contents API source.
func NewSource(httpClient *http.Client, tokenProvider TokenProvider, baseURL string) Source {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}

	return Source{
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
		baseURL:       baseURL,
	}
}

// FetchEncrypted reads encrypted bytes from GilmanLab/secrets with the raw Contents API media type.
func (s Source) FetchEncrypted(ctx context.Context, request appsecrets.Request) ([]byte, error) {
	token, err := s.tokenProvider.Token(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("obtain GitHub token: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("GitHub token is empty")
	}

	endpoint, err := s.endpoint(request)
	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create GitHub contents request: %w", err)
	}
	httpRequest.Header.Set("Accept", acceptRawHeader)
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("User-Agent", userAgent)
	httpRequest.Header.Set(apiVersionHeader, apiVersion)

	response, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("fetch encrypted secret from GitHub: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read GitHub contents response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(body))
		if len(message) > maxErrorBytes {
			message = message[:maxErrorBytes]
		}

		return nil, fmt.Errorf("GitHub contents request failed with status %d: %s", response.StatusCode, message)
	}

	return body, nil
}

func (s Source) endpoint(request appsecrets.Request) (string, error) {
	base, err := url.Parse(s.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse GitHub API base URL: %w", err)
	}

	segments := append(
		[]string{"repos", repoOwner, repoName, "contents"},
		strings.Split(request.Path, "/")...,
	)
	endpoint := base.JoinPath(segments...)
	values := endpoint.Query()
	values.Set("ref", request.Ref)
	endpoint.RawQuery = values.Encode()

	return endpoint.String(), nil
}
