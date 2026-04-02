package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

// Config описывает настройки
type Config struct {
	ApiKey       string
	ERP_tool_url string `json:"erp_tool_url"`
	ERP_call_url string `json:"erp_call_url"`
}

type ToolParam struct {
	Type        string               `json:"type"`
	Required    bool                 `json:"required"`
	Description string               `json:"description"`
	Items       map[string]ToolParam `json:"items,omitempty"` // для array
}

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
	if cfg.ApiKey == "" {
		log.Fatal("ERP_API_KEY not set. Please add env in the mcp client.")
	}

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
	req, err := http.NewRequest("POST", cfg.ERP_tool_url, nil)
	if err != nil {
		log.Fatal("HTTP Request not created.")
	}

	// Устанавливаем заголовок с типом данных в теле запроса
	req.Header.Set("Authorization", cfg.ApiKey)

	// Выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(req)
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

	// Устанавливаем заголовок с типом данных в теле запроса
	request.Header.Set("Authorization", cfg.ApiKey)
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
	// Возвращаем MCP результат
	return mcp.NewToolResultText(string(body)), nil
}
