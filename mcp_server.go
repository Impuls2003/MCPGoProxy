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

	// HTTP запускаем только если не указан --stdio-only.
	if !isStdioOnly() {
		go func() {
			if err := serveHTTP(s, cfg); err != nil {
				fmt.Println("HTTP server error:", err)
				os.Exit(1)
			}
		}()
	}

	if err := server.ServeStdio(s); err != nil {
		fmt.Println(err)
	}
}

func serveHTTP(s *server.MCPServer, cfg Config) error {
	httpServer := server.NewStreamableHTTPServer(s)

	return httpServer.Start(
		cfg.HTTPHost + ":" + strconv.Itoa(cfg.HTTPPort),
	)
}

func isStdioOnly() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--stdio-only" {
			return true
		}
	}

	return false
}
