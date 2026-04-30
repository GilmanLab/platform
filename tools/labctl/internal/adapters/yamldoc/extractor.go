package yamldoc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	documentNodeChildren = 1
	keyValueNodeWidth    = 2
	scalarLineEnding     = "\n"
)

// Extractor extracts RFC 6901 JSON Pointer fields from YAML documents.
type Extractor struct{}

// ExtractYAML renders a selected YAML field as raw scalar text or YAML for structured values.
func (e Extractor) ExtractYAML(ctx context.Context, document []byte, pointer string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var root yaml.Node
	if err := yaml.Unmarshal(document, &root); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	node, err := selectNode(&root, pointer)
	if err != nil {
		return nil, err
	}

	return renderNode(node)
}

func selectNode(root *yaml.Node, pointer string) (*yaml.Node, error) {
	if pointer == "" {
		return documentContent(root), nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("JSON Pointer must be empty or start with '/'")
	}

	node := documentContent(root)
	for rawToken := range strings.SplitSeq(pointer[1:], "/") {
		token, err := unescapeToken(rawToken)
		if err != nil {
			return nil, err
		}

		node, err = child(node, token)
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

func documentContent(root *yaml.Node) *yaml.Node {
	if root.Kind == yaml.DocumentNode && len(root.Content) == documentNodeChildren {
		return root.Content[0]
	}

	return root
}

func unescapeToken(token string) (string, error) {
	var out strings.Builder
	for idx := 0; idx < len(token); idx++ {
		if token[idx] != '~' {
			out.WriteByte(token[idx])
			continue
		}
		if idx+1 >= len(token) {
			return "", fmt.Errorf("invalid JSON Pointer escape in %q", token)
		}
		idx++
		switch token[idx] {
		case '0':
			out.WriteByte('~')
		case '1':
			out.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid JSON Pointer escape in %q", token)
		}
	}

	return out.String(), nil
}

func child(node *yaml.Node, token string) (*yaml.Node, error) {
	switch node.Kind {
	case yaml.MappingNode:
		return mappingChild(node, token)
	case yaml.SequenceNode:
		return sequenceChild(node, token)
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
		return nil, fmt.Errorf("cannot descend into YAML %s at %q", nodeKind(node), token)
	default:
		return nil, fmt.Errorf("cannot descend into YAML %s at %q", nodeKind(node), token)
	}
}

func mappingChild(node *yaml.Node, token string) (*yaml.Node, error) {
	for idx := 0; idx+1 < len(node.Content); idx += keyValueNodeWidth {
		if node.Content[idx].Value == token {
			return node.Content[idx+1], nil
		}
	}

	return nil, fmt.Errorf("field %q was not found", token)
}

func sequenceChild(node *yaml.Node, token string) (*yaml.Node, error) {
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 {
		return nil, fmt.Errorf("sequence token %q is not a valid array index", token)
	}
	if index >= len(node.Content) {
		return nil, fmt.Errorf("sequence index %d is out of range", index)
	}

	return node.Content[index], nil
}

func renderNode(node *yaml.Node) ([]byte, error) {
	if node.Kind == yaml.ScalarNode {
		return []byte(node.Value + scalarLineEnding), nil
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	if err := encoder.Encode(node); err != nil {
		_ = encoder.Close()

		return nil, fmt.Errorf("encode YAML field: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close YAML encoder: %w", err)
	}

	return out.Bytes(), nil
}

func nodeKind(node *yaml.Node) string {
	switch node.Kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "node"
	}
}
