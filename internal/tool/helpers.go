package tool

// GetString extracts a string value from Input
func GetString(in Input, key string) string {
	if v, ok := in[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt extracts an int value from Input with a default
func GetInt(in Input, key string, def int) int {
	if v, ok := in[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return def
}

// GetBool extracts a bool value from Input
func GetBool(in Input, key string) bool {
	if v, ok := in[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetStringSlice extracts a string slice from Input
func GetStringSlice(in Input, key string) []string {
	if v, ok := in[key]; ok {
		switch arr := v.(type) {
		case []string:
			return arr
		case []any:
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

// GetMap extracts a map from Input
func GetMap(in Input, key string) map[string]any {
	if v, ok := in[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}
