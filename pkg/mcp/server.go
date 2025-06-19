package mcp

import (
	// "gerrit-mcp/internal/gerrit"
	"gerrit-mcp/internal/change"
	"github.com/andygrunwald/go-gerrit"
	"gerrit-mcp/internal/logger"
	"gerrit-mcp/internal/util"
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
					"crbugID": {
						"type": "number",
						"description": "crbug ID"
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
	crbugID := mcp.ParseInt(request, "crbugID", -1)

	if reviewURL == "" && crbugID == -1 {
		return nil, fmt.Errorf("either reviewURL or crbugID must be provided")
	}

	if reviewURL != "" {
		return mcp.NewToolResultText("not implemented yet"), nil
	}

	// if crbugID != -1 {
	opt := &gerrit.QueryChangeOptions{}
	opt.Query = []string{fmt.Sprintf("tr:%d", crbugID)}
	changes, _, err := s.gerritClient.Changes.QueryChanges(ctx, opt)
	if err != nil {
		panic(err)
	}
	if len(*changes) == 0 {
		return nil, fmt.Errorf("no change found for crbugID %d", crbugID)
	}
	gerritChange := make([]change.GerritChange, 0)
	for _, curChange := range *changes {
		logger.Debugf("processing %s %s", curChange.ID, curChange.Subject)
		revision := curChange.CurrentRevision
		if revision == "" {
			revision = "current"
		}
		unfilteredFiles, _, rerr := s.gerritClient.Changes.ListFiles(ctx, curChange.ID, revision, &gerrit.FilesOptions{})
		if rerr != nil {
			// panic(rerr)
			logger.Errorf("%v", rerr)
			continue
		}
		files := util.FilterFiles(unfilteredFiles)
		logger.Debugf("filtered files count %d\n", len(files))
		// TODO: move to ShouldSkipChange
		if len(files) > 32 || len(files) == 0 {
			continue
		}
		logger.Debugf("moving with %s\n", strings.Join(files, "\n"))
		diffs := make([]*gerrit.DiffInfo, 0)
		for _, fname := range files {
			if fname == "/COMMIT_MSG" || fname == "/MERGE_LIST" || fname == "/PATCHSET_LEVEL" {
				continue
			}
			diffInfo, _, diffErr := s.gerritClient.Changes.GetDiff(ctx, curChange.ID, revision, fname, nil)
			if diffErr != nil {
				logger.Errorf("%v", diffErr)
			}
			diffs = append(diffs, diffInfo)
		}
		GerritChange, err := change.NewGerritChange(&curChange, diffs, DefaultGerritEndpointURL)
		if err != nil {
			logger.Errorf("%v", err)
		}
		gerritChange = append(gerritChange, GerritChange)
		// addErr := GerritChanger.AddChange(&change, diffs)
		// if addErr != nil {
		// 	logger.Errorf("%v", addErr)
		// }
	}
	logger.Debugf("extracted %d changes", len(gerritChange))

	resultBuilder := strings.Builder{}
	for _, GerritChange := range gerritChange {
		resultBuilder.WriteString(fmt.Sprintf("%s: %s\nChanged files: %s\n", GerritChange.URL, GerritChange.Subject, 
			strings.Join(GerritChange.Paths, "\n")))
		for fname, diff := range GerritChange.DiffMap {
			resultBuilder.WriteString(fmt.Sprintf("%s:\n%s\n", fname, diff))
		}
	}

	return mcp.NewToolResultText(resultBuilder.String()), nil
	// }
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
