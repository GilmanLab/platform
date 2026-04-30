package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	secretOutputFormatYAML = "yaml"
	secretOutputFormatJSON = "json"

	privateOutputFileMode = 0o600
)

func renderSecretData(format string, data []byte) ([]byte, error) {
	if err := validateSecretOutputFormat(format); err != nil {
		return nil, err
	}

	switch format {
	case "", secretOutputFormatYAML:
		return data, nil
	case secretOutputFormatJSON:
		return renderSecretJSON(data)
	}

	return nil, fmt.Errorf("invalid output format %q: expected yaml or json", format)
}

func validateSecretOutputFormat(format string) error {
	switch format {
	case "", secretOutputFormatYAML, secretOutputFormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid output format %q: expected yaml or json", format)
	}
}

func renderSecretJSON(data []byte) ([]byte, error) {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("parse decrypted YAML: %w", err)
	}

	normalized, err := normalizeYAMLValue(value)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	if err := encoder.Encode(normalized); err != nil {
		return nil, fmt.Errorf("encode decrypted JSON: %w", err)
	}
	return out.Bytes(), nil
}

func normalizeYAMLValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeYAMLStringMap(typed)
	case map[any]any:
		return normalizeYAMLAnyMap(typed)
	case []any:
		return normalizeYAMLSlice(typed)
	default:
		return value, nil
	}
}

func normalizeYAMLStringMap(value map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(value))
	for key, child := range value {
		normalizedChild, err := normalizeYAMLValue(child)
		if err != nil {
			return nil, err
		}
		normalized[key] = normalizedChild
	}
	return normalized, nil
}

func normalizeYAMLAnyMap(value map[any]any) (map[string]any, error) {
	normalized := make(map[string]any, len(value))
	for key, child := range value {
		keyString, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("convert YAML to JSON: mapping key %v has type %T, expected string", key, key)
		}
		normalizedChild, err := normalizeYAMLValue(child)
		if err != nil {
			return nil, err
		}
		normalized[keyString] = normalizedChild
	}
	return normalized, nil
}

func normalizeYAMLSlice(value []any) ([]any, error) {
	normalized := make([]any, len(value))
	for i, child := range value {
		normalizedChild, err := normalizeYAMLValue(child)
		if err != nil {
			return nil, err
		}
		normalized[i] = normalizedChild
	}
	return normalized, nil
}

func writeSecretData(stdout io.Writer, outputPath string, data []byte) error {
	if outputPath == "" || outputPath == "-" {
		_, err := stdout.Write(data)
		if err != nil {
			return fmt.Errorf("write secret output: %w", err)
		}
		return nil
	}

	if err := writePrivateOutputFile(outputPath, data); err != nil {
		return fmt.Errorf("write secret output: %w", err)
	}
	return nil
}

func writePrivateOutputFile(path string, data []byte) error {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("output parent directory %q: %w", parent, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output parent directory %q is not a directory", parent)
	}

	file, err := os.CreateTemp(parent, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary output file: %w", err)
	}
	tempPath := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if err := file.Chmod(privateOutputFileMode); err != nil {
		_ = file.Close()
		return fmt.Errorf("set temporary output file mode: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write temporary output file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary output file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace output file: %w", err)
	}

	cleanup = false

	if err := os.Chmod(path, privateOutputFileMode); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("set output file mode: %w", err)
	}
	return nil
}
