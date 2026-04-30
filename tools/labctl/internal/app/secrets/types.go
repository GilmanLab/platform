package secrets

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
)

const (
	// DefaultRef is the default Git ref used when callers do not select one.
	DefaultRef = "master"
)

const (
	// SourceAuto selects the local source when it is configured, then GitHub.
	SourceAuto SourceMode = "auto"
	// SourceLocal selects only the local secrets repository.
	SourceLocal SourceMode = "local"
	// SourceGitHub selects only the GitHub Contents API source.
	SourceGitHub SourceMode = "github"
)

// SourceMode selects where encrypted secret documents are fetched from.
type SourceMode string

// Request describes a reusable secret fetch request.
type Request struct {
	// Path is a repository-relative path inside GilmanLab/secrets.
	Path string
	// Ref is the Git ref used by remote fetches.
	Ref string
	// Source selects local, GitHub, or automatic source behavior.
	Source SourceMode
	// LocalRepoDir is an explicit local checkout path for the secrets repository.
	LocalRepoDir string
	// Field is an optional RFC 6901 JSON Pointer to extract after decryption.
	Field string
	// BrokerFunction is the AWS Lambda function name used for broker token minting.
	BrokerFunction string
	// AWSRegion is an optional AWS region override for broker invocation.
	AWSRegion string
}

// Result is the decrypted command-independent secret payload.
type Result struct {
	// Path is the validated repository-relative secret path.
	Path string
	// Ref is the Git ref used by the fetch.
	Ref string
	// Field is the selected JSON Pointer, when one was requested.
	Field string
	// Data is the full decrypted YAML document or extracted field rendering.
	Data []byte
}

// CleanPath validates and normalizes a repository-relative secret path.
func CleanPath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("secret path is required")
	}
	if path.IsAbs(raw) {
		return "", fmt.Errorf("secret path %q must be repository-relative", raw)
	}
	if strings.Contains(raw, `\`) {
		return "", fmt.Errorf("secret path %q must use forward slashes", raw)
	}
	if slices.Contains(strings.Split(raw, "/"), "..") {
		return "", fmt.Errorf("secret path %q must not contain '..'", raw)
	}

	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("secret path %q must be repository-relative", raw)
	}

	return cleaned, nil
}

func normalizeSourceMode(mode SourceMode) (SourceMode, error) {
	switch mode {
	case "", SourceAuto:
		return SourceAuto, nil
	case SourceLocal, SourceGitHub:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid secrets source %q: expected auto, local, or github", mode)
	}
}
