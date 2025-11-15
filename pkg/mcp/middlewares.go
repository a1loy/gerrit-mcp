package mcp

import (
	"context"
	"fmt"
	"gerrit-mcp/internal/logger"

	mcpserver "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolHandlerFunc = server.ToolHandlerFunc

type TokenValidator interface {
	Extract(ctx context.Context, req mcpserver.CallToolRequest) string
	Validate(token string) (any, error)
	IsDisabled() bool
}

type AuthMiddleware struct {
	tokenValidator TokenValidator
}

func NewAuthMiddleware(validator TokenValidator) *AuthMiddleware {
	if validator.IsDisabled() {
		logger.Infof("auth token validation disabled")
	}
	return &AuthMiddleware{tokenValidator: validator}
}

func (m *AuthMiddleware) ToolMiddleware() server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcpserver.CallToolRequest) (*mcpserver.CallToolResult, error) {
			token := m.tokenValidator.Extract(ctx, req)
			if token == "" {
				return nil, fmt.Errorf("authentication required")
			}
			// Validate token
			tokenScopes, err := m.tokenValidator.Validate(token)
			if err != nil {
				return nil, fmt.Errorf("invalid token: %w", err)
			}

			// Add user to context
			ctx = context.WithValue(ctx, "scopes", tokenScopes)

			return next(ctx, req)
		}
	}
}
