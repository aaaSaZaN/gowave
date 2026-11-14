package subtpl

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"gowave/internal/subtpl/ojson"

	"gopkg.in/yaml.v3"
)

// LoadYAML parses a template like js-yaml's load with upstream's
// YAML_MERGE_SCHEMA (CORE_SCHEMA plus "<<" merge keys).
func LoadYAML(text string) (any, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(text), &document); err != nil {
		return nil, err
	}
	if document.Kind == 0 || len(document.Content) == 0 {
		return nil, nil
	}
	return yamlValue(document.Content[0], 0)
}

func yamlValue(node *yaml.Node, depth int) (any, error) {
	if depth > 10000 {
		return nil, errors.New("yaml: nesting too deep")
	}
	switch node.Kind {
	case yaml.AliasNode:
		return yamlValue(node.Alias, depth+1)
	case yaml.ScalarNode:
		return yamlScalar(node)
	case yaml.SequenceNode:
		list := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := yamlValue(item, depth+1)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
		return list, nil
	case yaml.MappingNode:
		object := ojson.New()
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode, valueNode := node.Content[i], node.Content[i+1]
			if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "<<" && keyNode.Tag != "!!str" {
				if err := yamlMerge(object, valueNode, depth); err != nil {
					return nil, err
				}
				continue
			}
			key, err := yamlValue(keyNode, depth+1)
			if err != nil {
				return nil, err
			}
			value, err := yamlValue(valueNode, depth+1)
			if err != nil {
				return nil, err
			}
			object.Set(jsString(key), value)
		}
		return object, nil
	}
	return nil, nil
}

// yamlMerge applies "<<": merged keys never override keys set earlier in the
// mapping (js-yaml mergeMappings); later explicit keys override them.
func yamlMerge(target *ojson.Object, node *yaml.Node, depth int) error {
	sources := []*yaml.Node{node}
	resolved := node
	for resolved.Kind == yaml.AliasNode {
		resolved = resolved.Alias
	}
	if resolved.Kind == yaml.SequenceNode {
		sources = resolved.Content
	}
	for _, source := range sources {
		value, err := yamlValue(source, depth+1)
		if err != nil {
			return err
		}
		object, ok := value.(*ojson.Object)
		if !ok {
			return errors.New("yaml: cannot merge mappings; the provided source object is unacceptable")
		}
		for _, key := range object.Keys() {
			if !target.Has(key) {
				target.Set(key, object.Value(key))
			}
		}
	}
	return nil
}

func yamlScalar(node *yaml.Node) (any, error) {
	switch node.ShortTag() {
	case "!!null":
		return nil, nil
	case "!!bool":
		var b bool
		if err := node.Decode(&b); err != nil {
			return nil, err
		}
		return b, nil
	case "!!int":
		value := strings.ReplaceAll(node.Value, "_", "")
		if i, err := strconv.ParseInt(value, 0, 64); err == nil {
			return float64(i), nil
		}
		var f float64
		if err := node.Decode(&f); err != nil {
			return nil, err
		}
		return f, nil
	case "!!float":
		var f float64
		if err := node.Decode(&f); err != nil {
			return nil, err
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, nil // JSON round trip turns these into null
		}
		return f, nil
	}
	// js-yaml's CORE_SCHEMA has no timestamp type: dates stay strings.
	return node.Value, nil
}
