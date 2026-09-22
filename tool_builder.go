package main

// Поток формирования инструмента:
//
// 1. ERP возвращает описание инструмента в собственном формате.
// 2. JSON десериализуется в ToolSpec / ToolParam.
// 3. ToolSpec преобразуется в MCP Tool.
// 4. buildSchema / buildObjectSchema рекурсивно формируют JSON Schema.
// 5. MCP-клиент получает стандартный inputSchema.
//
// ERP JSON → ToolSpec → MCP Tool → JSON Schema → LLM

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// buildTool создает MCP-инструмент из промежуточного описания ToolSpec.
//
// Для каждого параметра создается соответствующий MCP ToolOption.
// Вложенная структура параметров преобразуется в JSON Schema рекурсивно.
func buildTool(spec ToolSpec) mcp.Tool {

	if len(spec.Parameters) == 0 {
		return mcp.NewToolWithRawSchema(
			spec.Name,
			spec.Description,
			json.RawMessage(`{
				"type": "object",
				"properties": {},
				"additionalProperties": false
			}`),
		)
	}

	opts := []mcp.ToolOption{
		mcp.WithDescription(spec.Description),
	}

	for paramName, param := range spec.Parameters {
		opts = append(opts, buildToolParam(paramName, param))
	}

	return mcp.NewTool(spec.Name, opts...)
}

// buildToolParam преобразует один параметр ToolParam в MCP ToolOption.
//
// Простые типы (string, number, boolean) преобразуются непосредственно.
// Для array дополнительно формируется описание элементов массива.
//
// Если массив содержит объекты, их структура строится через
// buildObjectSchema.
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
		opts = append(opts, mcp.Items(buildSchemaItems(param)))
		return mcp.WithArray(name, opts...)

	default:
		panic("unsupported param type: " + param.Type)
	}
}

// buildSchema преобразует ToolParam во внутреннее представление JSON Schema.
//
// Функция рекурсивная: если параметр является массивом объектов,
// для описания элементов массива вызывается buildObjectSchema, который,
// в свою очередь, снова может вызвать buildSchema.
//
// Это позволяет описывать структуры произвольной вложенности:
//
// array
//
//	-> object
//	   -> array
//	      -> object
//	         -> ...
func buildSchema(param ToolParam) map[string]any {
	schema := map[string]any{
		"type": param.Type,
	}

	if param.Description != "" {
		schema["description"] = param.Description
	}

	switch param.Type {

	case "array":
		if param.ItemType != "" {
			schema["items"] = map[string]any{
				"type": param.ItemType,
			}
		} else {
			schema["items"] = buildObjectSchema(param.Items)
		}

	case "object":
		objectSchema := buildObjectSchema(param.Items)

		for key, value := range objectSchema {
			schema[key] = value
		}
	}

	return schema
}

// buildObjectSchema строит JSON Schema объекта из набора его полей.
//
// Каждое поле преобразуется через buildSchema, поэтому функция корректно
// обрабатывает не только простые поля, но и вложенные массивы и объекты.
//
// Обязательные поля дополнительно помещаются в массив required.
func buildObjectSchema(fields map[string]ToolParam) map[string]any {
	properties := map[string]any{}
	required := []string{}

	for name, field := range fields {
		properties[name] = buildSchema(field)

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

func buildSchemaItems(param ToolParam) map[string]any {
	if param.ItemType != "" {
		return map[string]any{
			"type": param.ItemType,
		}
	}

	return buildObjectSchema(param.Items)
}
