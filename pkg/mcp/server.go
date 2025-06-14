package mcp

import (
	"gerrit-mcp/internal/gerrit"
	"gerrit-mcp/internal/logger"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ServerName = "gerrit-mcp"
	ServerVersion = "0.1.0"
	DefaultGerritEndpointURL = "https://chromium-review.googlesource.com"
)

type Server struct {
	mcpServer *server.MCPServer
	gerritClient *gerrit.Client
}

func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		gerritClient: gerrit.NewClient(DefaultGerritEndpointURL),
	}

	for _, opt := range opts {
		opt(s)
	}

	mcpServer := server.NewMCPServer(ServerName, ServerVersion)

	s.mcpServer = mcpServer
	return s
}

type ServerOption func(*Server)

func WithGerritClient(client *gerrit.Client) ServerOption {
	return func(s *Server) {
		s.gerritClient = client
	}
}

func (s *Server) ServeSSE(addr string) error {
	logger.Debugf("Starting MCP server (SSE) on %s", addr)
	sseServer := server.NewSSEServer(s.mcpServer)
	return sseServer.Start(addr)
}
