package config

// mergeMaps performs a deep merge of src into dst. Values in src win.
// Nested maps are merged recursively; all other types are overwritten.
func mergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}
	for k, srcVal := range src {
		dstVal, exists := dst[k]
		if !exists {
			dst[k] = srcVal
			continue
		}
		dstMap, dstOk := dstVal.(map[string]any)
		srcMap, srcOk := srcVal.(map[string]any)
		if dstOk && srcOk {
			dst[k] = mergeMaps(dstMap, srcMap)
		} else {
			dst[k] = srcVal
		}
	}
	return dst
}
