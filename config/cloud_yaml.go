package config

import (
	"bytes"
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// WithCloudBlock returns the config YAML with the cloud block set to the given
// values. Other keys and comments stay as they are. An empty input yields a
// config with only the cloud block. A config that still lists orgs or repos
// is rejected because cloud mode replaces them.
func WithCloudBlock(existing []byte, cloud *CloudConfig) ([]byte, error) {
	var document yaml.Node
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := yaml.Unmarshal(existing, &document); err != nil {
			return nil, fmt.Errorf("parse existing config: %w", err)
		}
	}
	root := documentMapping(&document)
	if root == nil {
		return nil, errors.New("existing config is not a YAML mapping")
	}
	for index := 0; index+1 < len(root.Content); index += 2 {
		key := root.Content[index].Value
		if key == "orgs" || key == "repos" {
			return nil, fmt.Errorf("config still has a %q block, cloud mode replaces orgs and repos, remove them first", key)
		}
	}

	var cloudNode yaml.Node
	if err := cloudNode.Encode(cloud); err != nil {
		return nil, fmt.Errorf("encode cloud block: %w", err)
	}
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == "cloud" {
			root.Content[index+1] = &cloudNode
			return yaml.Marshal(&document)
		}
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "cloud"},
		&cloudNode,
	)
	return yaml.Marshal(&document)
}

// documentMapping returns the top level mapping of the document, creating an
// empty one when the document has no content yet.
func documentMapping(document *yaml.Node) *yaml.Node {
	if document.Kind == 0 {
		mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		document.Kind = yaml.DocumentNode
		document.Content = []*yaml.Node{mapping}
		return mapping
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}
