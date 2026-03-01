package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// RenderYAML marshals a values map to YAML bytes suitable for a ConfigMap.
func RenderYAML(values map[string]any) ([]byte, error) {
	if len(values) == 0 {
		return []byte("{}\n"), nil
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal values: %w", err)
	}
	return data, nil
}
