package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Создание MCP сервера
	s := server.NewMCPServer(
		"Dynamic MCP Proxy to 1C",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Загружаем конфигурацию
	cfg := LoadConfig()

	handler := func(cfg Config) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return CallToolsFromERP(cfg, ctx, req)
		}
	}(cfg) // захватываем cfg

	tools := GetToolsFromERP(cfg)

	for _, spec := range tools {
		tool := buildTool(spec)
		s.AddTool(tool, handler)
	}

	if err := server.ServeStdio(s); err != nil {
		fmt.Println(err)
	}
}
