package mcp

import (
	// "gerrit-mcp/internal/gerrit"
	"github.com/andygrunwald/go-gerrit"
	"gerrit-mcp/internal/logger"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mark3labs/mcp-go/mcp"
	"context"
	"strings"
	"fmt"
	"encoding/json"
)

const (
	ServerName = "gerrit-mcp"
	ServerVersion = "0.1.0"
	DefaultGerritEndpointURL = "https://chromium-review.googlesource.com"
	PROJECT_QUERY_LIMIT = 10
)

type Server struct {
	mcpServer *server.MCPServer
	gerritClient *gerrit.Client
}

func NewServer(opts ...ServerOption) *Server {
	ctx := context.Background()
	client, err := gerrit.NewClient(ctx, DefaultGerritEndpointURL, nil)
	if err != nil {
		panic(err)
	}
	s := &Server{
		gerritClient: client,
	}

	for _, opt := range opts {
		opt(s)
	}

	mcpServer := server.NewMCPServer(ServerName, ServerVersion)

	mcpServer.AddTool(
		mcp.NewToolWithRawSchema(
			"query_projects",
			"Query available projects",
			json.RawMessage(`{
				"type": "object",
				"properties": {
					"prefix": {
						"type": "string",
						"description": "Project name prefix"
					},
					"limit": {
						"type": "number",
						"description": "Number of projects to return"
					}
				},
				"required": []
			}`),
		),
		s.handleQueryProjects,
	)

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

func (s *Server) handleQueryProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prefix := mcp.ParseString(request, "prefix", "")
	limit := mcp.ParseInt(request, "limit", PROJECT_QUERY_LIMIT)
	opt := &gerrit.ProjectOptions{
		ProjectBaseOptions: gerrit.ProjectBaseOptions{
			Limit: limit,
		},	
		Description: true,
	}
	if prefix != "" {
		opt.Prefix = prefix
	}
	projects, _, err := s.gerritClient.Projects.ListProjects(ctx, opt)
	if err != nil {
		return nil, err
	}
	// TODO: better response representation
	resultBuilder := strings.Builder{}
	for name, project := range *projects {
		logger.Debugf("Found project: %s", name)
		resultBuilder.WriteString(fmt.Sprintf("%s: %s\n", name, project.Description))
	}
	return mcp.NewToolResultText(resultBuilder.String()), nil
}
