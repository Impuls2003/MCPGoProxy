package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Создание MCP сервера
	s := server.NewMCPServer(
		"Dynamic MCP Proxy to 1C",
		"1.1.0",
		server.WithToolCapabilities(false),
		server.WithInputSchemaValidation(),
	)

	// Загружаем конфигурацию
	cfg := LoadConfig()

	handler := createToolHandler(cfg) // захватываем cfg

	tools := GetToolsFromERP(cfg)

	for _, spec := range tools {
		tool := buildTool(spec)
		s.AddTool(tool, handler) // Для всех инструментов делаем один обработчик. Всю магию проксирования будет выполнять CallToolsFromERP
	}

	// Определяем режим запуска
	switch getRunMode() {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			fmt.Println(err)
		}
	case "http":
		if err := serveHTTP(s, cfg); err != nil {
			fmt.Println("HTTP server error:", err)
			os.Exit(1)
		}
	}
}

// createToolHandler создает обработчик вызовов MCP-инструментов,
// привязанный к конфигурации прокси.
//
// Конфигурация замыкается во внутренней функции и поэтому
// доступна при последующих вызовах MCP без передачи ее в запросе.
// Упрощенно:
//
//	cfg → handler(cfg) → func(ctx, req) → CallToolsFromERP(cfg, ctx, req)
//
// Вызов "(cfg)" в конце сразу выполняет внешнюю функцию и получает
// готовый обработчик.
func createToolHandler(cfg Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return CallToolsFromERP(cfg, ctx, req)
	}
}

func serveHTTP(s *server.MCPServer, cfg Config) error {
	httpServer := server.NewStreamableHTTPServer(
		s,
		server.WithDisableLocalhostProtection(cfg.DisableLocalhostProtection),
	)

	addr := cfg.HTTPHost + ":" + strconv.Itoa(cfg.HTTPPort)

	fmt.Println("Streamable HTTP server:", addr+"/mcp")

	return httpServer.Start(addr)
}

func getRunMode() string {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--stdio":
			return "stdio"
		case "--http":
			return "http"
		}
	}

	return "stdio"
}
