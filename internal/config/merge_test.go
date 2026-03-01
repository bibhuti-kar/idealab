package config

import (
	"testing"
)

func TestMergeMaps_FlatOverride(t *testing.T) {
	dst := map[string]any{"a": "old", "b": 1}
	src := map[string]any{"a": "new", "c": 2}
	result := mergeMaps(dst, src)

	if result["a"] != "new" {
		t.Errorf("expected a=new, got %v", result["a"])
	}
	if result["b"] != 1 {
		t.Errorf("expected b=1, got %v", result["b"])
	}
	if result["c"] != 2 {
		t.Errorf("expected c=2, got %v", result["c"])
	}
}

func TestMergeMaps_DeepMerge(t *testing.T) {
	dst := map[string]any{
		"nested": map[string]any{
			"keep": "yes",
			"over": "old",
		},
	}
	src := map[string]any{
		"nested": map[string]any{
			"over": "new",
			"add":  "added",
		},
	}
	result := mergeMaps(dst, src)

	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatal("nested is not a map")
	}
	if nested["keep"] != "yes" {
		t.Errorf("expected keep=yes, got %v", nested["keep"])
	}
	if nested["over"] != "new" {
		t.Errorf("expected over=new, got %v", nested["over"])
	}
	if nested["add"] != "added" {
		t.Errorf("expected add=added, got %v", nested["add"])
	}
}

func TestMergeMaps_NilDst(t *testing.T) {
	src := map[string]any{"key": "val"}
	result := mergeMaps(nil, src)

	if result["key"] != "val" {
		t.Errorf("expected key=val, got %v", result["key"])
	}
}

func TestMergeMaps_NilSrc(t *testing.T) {
	dst := map[string]any{"key": "val"}
	result := mergeMaps(dst, nil)

	if result["key"] != "val" {
		t.Errorf("expected key=val, got %v", result["key"])
	}
}
