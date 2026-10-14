package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Config описывает настройки
type Config struct {
	ApiKey                     string
	ERP_tool_url               string            `json:"erp_tool_url"`
	ERP_call_url               string            `json:"erp_call_url"`
	HTTPHost                   string            `json:"http_host"`
	HTTPPort                   int               `json:"http_port"`
	DisableLocalhostProtection bool              `json:"disable_localhost_protection"`
	UserHeaders                map[string]string `json:"user_headers"`
	ForwardHeaders             []string          `json:"forward_headers"`
}

// ToolParam описывает один параметр инструмента в промежуточном формате,
// полученном из ERP.
//
// Структура намеренно не является JSON Schema. Это внутреннее представление,
// которое используется MCP-прокси для преобразования описания инструмента,
// полученного от 1С, в MCP inputSchema.
//
// Для простых типов используется Type.
// Для массивов:
//   - ItemType — тип элементов простого массива;
//   - Items — описание полей объекта, если элементы массива являются объектами.
//
// Items может содержать вложенные ToolParam, поэтому структура поддерживает
// массивы объектов и другие вложенные структуры.
type ToolParam struct {
	Type        string               `json:"type"`
	Required    bool                 `json:"required"`
	Description string               `json:"description"`
	Items       map[string]ToolParam `json:"items,omitempty"` // для array
	ItemType    string               `json:"itemType"`        // для array
}

// ToolSpec описывает MCP-инструмент в промежуточном представлении.
//
// Description и Parameters формируются на основании описания инструмента,
// полученного от ERP.
type ToolSpec struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Parameters  map[string]ToolParam `json:"parameters"`
}

// Загружаем конфигурацию
func LoadConfig() Config {

	// Определяем путь к установочному файлу
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal("Unable to determine path to exe:", err)
	}

	configPath := filepath.Join(filepath.Dir(exePath), "config.json")

	// Ищем config.json
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatal("config.json file not found.")
	}
	defer file.Close()

	// Читаем config.json
	var cfg Config

	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		log.Fatal("Error reading config.json:", err)
	}

	// apiKey нужно получить от MCP Клиента (LMStudio?)
	cfg.ApiKey = os.Getenv("ERP_API_KEY")

	// Проверяем чтобы все переменные были загружены из config

	if cfg.ERP_tool_url == "" {
		log.Fatal("erp_tool_url not set.")
	}

	if cfg.ERP_call_url == "" {
		log.Fatal("erp_call_url not set.")
	}

	return cfg
}

func GetToolsFromERP(cfg Config) []ToolSpec {

	// Создаем новый запрос
	request, err := http.NewRequest("POST", cfg.ERP_tool_url, nil)
	if err != nil {
		log.Fatal("HTTP Request not created.")
	}

	// Устанавливаем заголовок с типом данных в теле запроса
	request.Header.Set("Authorization", cfg.ApiKey)

	// Добавляем пользовательские заголовки
	addUserHeadersToRequest(request, cfg)

	// Выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		log.Fatal("HTTP Request not created.")
	}
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	// Выводим ответ от сервера
	if resp.StatusCode != 200 {
		log.Fatal("HTTP Request" + resp.Status + string(body))
		return nil
	}

	// Парсим пришедший от 1с JSON в спецификацию для mcp-go
	var erpTools []ToolSpec
	err = json.Unmarshal([]byte(body), &erpTools)
	if err != nil {
		log.Fatal("Error parsing JSON tools:", err)
	}

	return erpTools
}

func CallToolsFromERP(cfg Config, ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	// Формируем payload
	payload := map[string]interface{}{
		"tool":      req.Params.Name,
		"arguments": req.GetArguments(),
	}

	// Сериализуем в JSON
	data, _ := json.Marshal(payload)

	// Создаем новый запрос
	request, err := http.NewRequest("POST", cfg.ERP_call_url, bytes.NewBuffer(data))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Добавляем пользовательские заголовки
	addUserHeadersToRequest(request, cfg)

	// Ретранслируем заголовки
	addHeadersToRequest(request, cfg, req.Header)

	request.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	// Выводим ответ от сервера
	if resp.StatusCode != 200 {
		return mcp.NewToolResultError(string(body)), nil
	}

	// Получаем переданный Content-Type. По умолчанию octet-stream
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || contentType == "" {
		contentType = "application/octet-stream"
	}
	fileName := "unnamed_file"

	// Имя файла
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		if name := params["filename"]; name != "" {
			fileName = name
		}
	}

	// От Content-Type зависит как мы будем возвращать результат модели
	// Возвращаем MCP результат
	switch {
	case strings.HasPrefix(contentType, "image/"):
		imageData := base64.StdEncoding.EncodeToString(body)
		return mcp.NewToolResultImage(fileName, imageData, contentType), nil

	case contentType == "application/json" || strings.HasSuffix(contentType, "+json"):
		return mcp.NewToolResultJSON(json.RawMessage(body))

	case strings.HasPrefix(contentType, "text/"):
		return mcp.NewToolResultText(string(body)), nil

	default:
		fileData := base64.StdEncoding.EncodeToString(body)

		return mcp.NewToolResultResource(
			fileName,
			mcp.BlobResourceContents{
				URI:      "erp://files/" + url.PathEscape(fileName),
				MIMEType: contentType,
				Blob:     fileData,
			},
		), nil
	}
}

func addHeadersToRequest(request *http.Request, cfg Config, incomingHeaders http.Header) {
	// 1. Устанавливаем резервный API-ключ из конфигурации.
	if cfg.ApiKey != "" {
		request.Header.Set("Authorization", cfg.ApiKey)
	}

	// 2. Пересылаем только разрешённые заголовки.
	for _, headerName := range cfg.ForwardHeaders {
		values := incomingHeaders.Values(headerName)
		if len(values) == 0 {
			continue
		}

		// Не заменяем резервное значение пустым заголовком.
		var nonEmptyValues []string
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				nonEmptyValues = append(nonEmptyValues, value)
			}
		}

		if len(nonEmptyValues) > 0 {
			request.Header.Del(headerName)
			for _, value := range nonEmptyValues {
				request.Header.Add(headerName, value)
			}
		}
	}
}

func addUserHeadersToRequest(request *http.Request, cfg Config) {

	for name, value := range cfg.UserHeaders {
		request.Header.Set(name, value)
	}

}
