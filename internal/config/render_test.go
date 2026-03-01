package config

import (
	"strings"
	"testing"
)

func TestRenderYAML_Simple(t *testing.T) {
	values := map[string]any{
		"key": "value",
		"num": 42,
	}
	data, err := RenderYAML(values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "key: value") {
		t.Errorf("expected 'key: value' in output:\n%s", s)
	}
	if !strings.Contains(s, "num: 42") {
		t.Errorf("expected 'num: 42' in output:\n%s", s)
	}
}

func TestRenderYAML_Nested(t *testing.T) {
	values := map[string]any{
		"parent": map[string]any{
			"child": "val",
		},
	}
	data, err := RenderYAML(values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "parent:") {
		t.Errorf("expected 'parent:' in output:\n%s", s)
	}
	if !strings.Contains(s, "child: val") {
		t.Errorf("expected 'child: val' in output:\n%s", s)
	}
}

func TestRenderYAML_Empty(t *testing.T) {
	data, err := RenderYAML(map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "{}\n" {
		t.Errorf("expected empty YAML '{}\\n', got %q", string(data))
	}
}
