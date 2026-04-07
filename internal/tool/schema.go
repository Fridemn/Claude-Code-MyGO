package tool

// SchemaObject creates an object schema with properties and required fields
func SchemaObject(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// SchemaString creates a string schema with description
func SchemaString(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

// SchemaInteger creates an integer schema with description
func SchemaInteger(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
	}
}

// SchemaBoolean creates a boolean schema with description
func SchemaBoolean(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}

// SchemaArray creates an array schema with items and description
func SchemaArray(description string, items map[string]any) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       items,
	}
}

// SchemaEnumString creates a string schema with enum values
func SchemaEnumString(description string, values ...string) map[string]any {
	schema := SchemaString(description)
	if len(values) > 0 {
		schema["enum"] = values
	}
	return schema
}

// SchemaAnyObject creates an object schema that accepts any properties
func SchemaAnyObject(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": true,
	}
}

// SchemaUnion creates a union type schema (oneOf)
func SchemaUnion(description string, schemas ...map[string]any) map[string]any {
	return map[string]any{
		"description": description,
		"oneOf":       schemas,
	}
}
