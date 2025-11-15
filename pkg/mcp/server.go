package mcp

import (
	// "gerrit-mcp/internal/gerrit"
	"context"
	"encoding/json"
	"fmt"
	"gerrit-mcp/internal/change"
	"gerrit-mcp/internal/logger"
	"gerrit-mcp/internal/middlewares"
	"net/url"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const (
	ServerName                 = "gerrit-mcp"
	ServerVersion              = "0.1.0"
	DefaultGerritEndpointURL   = "https://chromium-review.googlesource.com"
	ProjectQueryLimit          = 10
	ChangeQueryDefaultAgeHours = 24
	ChangeQueryDefaultProject  = "chromium/src"
	ChangeQueryDefaultStatus   = "open"
	ChangeQueryDefaultLimit    = -1 // unlimited
	DefaultHeaderAuthName      = "Authorization"
)

type Server struct {
	mcpServer    *mcpserver.MCPServer
	gerritClient *gerrit.Client
	config       Config
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

	authMiddleware := NewAuthMiddleware(&middlewares.SimpleTokenValidator{HeaderName: s.config.AuthHeaderName, Secret: s.config.AuthSecret})

	mcpServer := mcpserver.NewMCPServer(ServerName, ServerVersion,
		mcpserver.WithToolHandlerMiddleware(authMiddleware.ToolMiddleware()))

	mcpServer.AddTool(
		mcp.NewToolWithRawSchema(
			"query_changes_by_filter",
			"Query changes by filter",
			json.RawMessage(`{
				"type": "object",
				"properties": {
					"status": {
						"type": "string",
						"description": "Status of the change"
					},
					"limit": {
						"type": "number",
						"description": "Number of changes to return"
					},
					"project": {
						"type": "string",
						"description": "Project name"
					},
					"age": {
						"type": "number",
						"description": "Age of the change in hours"
					}
				},
				"required": []
			}`),
		),
		s.handleQueryChangesByFilter,
	)

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

func WithConfig(cfg Config) ServerOption {
	return func(s *Server) {
		s.config = cfg
	}
}

func (s *Server) ServeSSE(addr string) error {
	logger.Debugf("Starting MCP server (SSE) on %s", addr)
	sseServer := server.NewSSEServer(s.mcpServer)
	return sseServer.Start(addr)
}

func (s *Server) handleQueryChangesByFilter(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status := mcp.ParseString(request, "status", ChangeQueryDefaultStatus)
	limit := mcp.ParseInt(request, "limit", ChangeQueryDefaultLimit)
	project := mcp.ParseString(request, "project", ChangeQueryDefaultProject)
	age := mcp.ParseInt(request, "age", ChangeQueryDefaultAgeHours)

	opt := &gerrit.QueryChangeOptions{}
	queryParts := []string{
		"status:" + status,
		// "project:" + project,
	}
	if project == ChangeQueryDefaultProject {
		queryParts = append(queryParts, "project:"+project)
	} else {
		queryParts = append(queryParts, "project:"+change.GetCorrectProjectName(ctx, s.gerritClient, project, ChangeQueryDefaultProject))
	}

	if age != ChangeQueryDefaultAgeHours {
		queryParts = append(queryParts, fmt.Sprintf("age:%dh", age))
	}
	opt.Query = []string{strings.Join(queryParts, " ")}
	opt.Limit = limit
	changes, _, err := s.gerritClient.Changes.QueryChanges(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to query changes: %w", err)
	}

	if len(*changes) == 0 {
		return nil, fmt.Errorf("no change found for query %s", opt.Query[0])
	}

	gerritChanges, err := change.BuildGerritChanges(ctx, s.gerritClient, changes)
	if err != nil {
		return nil, err
	}

	logger.Debugf("extracted %d changes", len(gerritChanges))

	if limit != ChangeQueryDefaultLimit && len(gerritChanges) > limit {
		gerritChanges = gerritChanges[:limit]
	}

	resultBuilder := strings.Builder{}
	for _, gc := range gerritChanges {
		resultBuilder.WriteString(gc.TextResult())
	}
	return mcp.NewToolResultText(resultBuilder.String()), nil
}

func (s *Server) handleQueryChange(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reviewURL := mcp.ParseString(request, "reviewURL", "")
	trackID := mcp.ParseInt(request, "trackID", -1)

	if reviewURL == "" && trackID == -1 {
		return nil, fmt.Errorf("either reviewURL or trackID must be provided")
	}

	opt := &gerrit.QueryChangeOptions{}
	if reviewURL != "" {
		query, err := change.BuildQueryFromURL(reviewURL)
		reviewU, _ := url.Parse(reviewURL)
		if err != nil {
			return nil, fmt.Errorf("unable to parse review URL: %s: %v", reviewURL, err)
		}
		u := s.gerritClient.BaseURL()
		clientHost := u.Hostname()
		if clientHost != reviewU.Hostname() {
			logger.Errorf("host %s from review URL %s is not the same as the host %s from the server", reviewU.Hostname(), reviewURL, clientHost)
			return nil, fmt.Errorf("review URL is not from the same gerrit instance as the one used to create the server")
		}
		if err != nil {
			return nil, fmt.Errorf("unable to parse review URL: %s", reviewURL)
		}
		opt.Query = []string{query}
	} else {
		opt.Query = []string{fmt.Sprintf("tr:%d", trackID)}
	}

	changes, _, err := s.gerritClient.Changes.QueryChanges(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to query changes: %w", err)
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
		resultBuilder.WriteString(gc.TextResult())
	}
	return mcp.NewToolResultText(resultBuilder.String()), nil
}

func (s *Server) handleQueryProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prefix := mcp.ParseString(request, "prefix", "")
	limit := mcp.ParseInt(request, "limit", ChangeQueryDefaultLimit)
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
