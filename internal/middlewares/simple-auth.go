package middlewares

import (
	"context"
	"fmt"

	mcpserver "github.com/mark3labs/mcp-go/mcp"
)

type SimpleTokenValidator struct {
	HeaderName string
	Secret     string
}

func (s *SimpleTokenValidator) IsDisabled() bool {
	return s.Secret == ""
}

func (s *SimpleTokenValidator) Extract(ctx context.Context, req mcpserver.CallToolRequest) string {
	reqHeaders := req.Header
	return reqHeaders.Get(s.HeaderName)
}

func (s *SimpleTokenValidator) Validate(token string) (any, error) {
	if s.Secret == "" {
		return nil, nil
	}
	if token == fmt.Sprintf("Bearer %s", s.Secret) {
		return "", nil
	}
	return nil, fmt.Errorf("invalid token")
}
