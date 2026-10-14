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
	// Проверяем возможно были команды установки службы windows
	switch getServiceCommand() {
	case "install":
		if err := installWindowsService(); err != nil {
			fmt.Println("Service installation error:", err)
			os.Exit(1)
		}
		fmt.Println("Windows service installed and started.")
		return

	case "uninstall":
		if err := removeWindowsService(); err != nil {
			fmt.Println("Service removal error:", err)
			os.Exit(1)
		}
		fmt.Println("Windows service removed.")
		return
	}

	// Проверяем возможность запуска как службы
	isService, err := runAsWindowsService()
	if err != nil {
		fmt.Println("Windows service error:", err)
		os.Exit(1)
	}
	if isService {
		return
	}

	// Загружаем конфигурацию
	cfg := LoadConfig()

	// Создаем mcp сервер
	s := newMCPServer(cfg)

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

func newMCPServer(cfg Config) *server.MCPServer {
	// Создание MCP сервера
	s := server.NewMCPServer(
		"Dynamic MCP Proxy to 1C",
		"1.1.1",
		server.WithToolCapabilities(false),
		server.WithInputSchemaValidation(),
	)

	handler := createToolHandler(cfg) // захватываем cfg

	tools := GetToolsFromERP(cfg)

	for _, spec := range tools {
		tool := buildTool(spec)
		s.AddTool(tool, handler) // Для всех инструментов делаем один обработчик. Всю магию проксирования будет выполнять CallToolsFromERP
	}

	return s
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

func newHTTPServer(s *server.MCPServer, cfg Config) *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(
		s,
		server.WithDisableLocalhostProtection(cfg.DisableLocalhostProtection),
	)
}

func serveHTTP(s *server.MCPServer, cfg Config) error {

	httpServer := newHTTPServer(s, cfg)

	addr := cfg.HTTPHost + ":" + strconv.Itoa(cfg.HTTPPort)
	fmt.Println("Streamable HTTP server:", addr+"/mcp")

	return httpServer.Start(addr)
}

func getServiceCommand() string {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--install-service":
			return "install"
		case "--uninstall-service":
			return "uninstall"
		}
	}
	return ""
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
