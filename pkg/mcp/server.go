package mcp

import (
	// "gerrit-mcp/internal/gerrit"
	"gerrit-mcp/internal/change"
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

	mcpServer.AddTool(
		mcp.NewToolWithRawSchema(
			"query_change",
			"Query particular change",
			json.RawMessage(`{
				"type": "object",
				"properties": {
					"reviewURL": {
						"type": "string",
						"description": "Review URL"
					},
					"trackID": {
						"type": "number",
						"description": "track ID (crbug ID in case of chromium)"
					}
				},
				"required": []
			}`),
		),
		s.handleQueryChange,
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

func (s *Server) handleQueryChange(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reviewURL := mcp.ParseString(request, "reviewURL", "")
	trackID := mcp.ParseInt(request, "trackID", -1)

	if reviewURL == "" && trackID == -1 {
		return nil, fmt.Errorf("either reviewURL or trackID must be provided")
	}

	if reviewURL != "" {
		return mcp.NewToolResultText("not implemented yet"), nil
	}

	opt := &gerrit.QueryChangeOptions{}
	opt.Query = []string{fmt.Sprintf("tr:%d", trackID)}
	changes, _, err := s.gerritClient.Changes.QueryChanges(ctx, opt)
	if err != nil {
		panic(err)
	}
	if len(*changes) == 0 {
		return nil, fmt.Errorf("no change found for trackID %d", trackID)
	}

	gerritChanges, err := change.BuildGerritChanges(ctx, s.gerritClient, changes)
	if err != nil {
		return nil, err
	}

	logger.Debugf("extracted %d changes", len(gerritChanges))

	resultBuilder := strings.Builder{}
	for _, gc := range gerritChanges {
		resultBuilder.WriteString(fmt.Sprintf("%s: %s\nChanged files: %s\n", gc.URL, gc.Subject, 
			strings.Join(gc.Paths, "\n")))
		for fname, diff := range gc.DiffMap {
			resultBuilder.WriteString(fmt.Sprintf("%s:\n%s\n", fname, diff))
		}
	}
	return mcp.NewToolResultText(resultBuilder.String()), nil
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
