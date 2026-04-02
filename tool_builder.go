package main

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// Функция создания инструмента
// создаем массив параметров и создаем инструмент
func buildTool(spec ToolSpec) mcp.Tool {

	opts := []mcp.ToolOption{
		mcp.WithDescription(spec.Description),
	}

	for paramName, param := range spec.Parameters {
		opts = append(opts, buildToolParam(paramName, param))
	}

	return mcp.NewTool(spec.Name, opts...)
}

// Создаем параметр по описанию
func buildToolParam(name string, param ToolParam) mcp.ToolOption {
	var opts []mcp.PropertyOption

	if param.Description != "" {
		opts = append(opts, mcp.Description(param.Description))
	}
	if param.Required {
		opts = append(opts, mcp.Required())
	}

	switch param.Type {

	case "string":
		return mcp.WithString(name, opts...)

	case "integer", "number":
		return mcp.WithNumber(name, opts...)

	case "boolean":
		return mcp.WithBoolean(name, opts...)

	case "array":
		// array of objects
		itemsSchema := buildObjectSchema(param.Items)
		opts = append(opts, mcp.Items(itemsSchema))
		return mcp.WithArray(name, opts...)

	case "object":
		//return mcp.WithObject(name, buildObjectOptions(param.Items)...)

	default:
		panic("unsupported param type: " + param.Type)
	}
	return nil
}

func buildObjectSchema(fields map[string]ToolParam) map[string]any {
	properties := map[string]any{}
	required := []string{}

	// Формируем inputSchema для
	for name, field := range fields {
		prop := map[string]any{
			"type": field.Type,
		}

		if field.Description != "" {
			prop["description"] = field.Description
		}

		properties[name] = prop

		if field.Required {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}
