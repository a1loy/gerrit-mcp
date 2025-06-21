package main

import (
	"flag"
	"fmt"
	"gerrit-mcp/internal/logger"	
	"gerrit-mcp/pkg/mcp"
	"os"
	"os/signal"
	"syscall"
	"context"
	"github.com/andygrunwald/go-gerrit"
)

const (
	DEFAULT_HOST = "0.0.0.0"
	DEFAULT_PORT = "8080"
	DEFAULT_GERRIT_INSTANCE = "https://chromium-review.googlesource.com"
)

func main() {
	port := flag.String("port", DEFAULT_PORT, "Port to listen on")
	addr := flag.String("addr", DEFAULT_HOST, "Address to listen on")
	gerritInstance := flag.String("gerrit-instance", DEFAULT_GERRIT_INSTANCE, "Gerrit instance URL")
	withAuth := flag.String("with-auth", "" , "Use authentication")
	flag.Parse()
	host := fmt.Sprintf("%s:%s", *addr, *port)
	logger.Debugf("Starting Gerrit MCP server on %s", host)
	logger.Debugf("Gerrit instance: %s", *gerritInstance)

	ctx := context.Background()
	gerritClient, err := gerrit.NewClient(ctx, *gerritInstance, nil)
	authMode := *withAuth
	switch authMode {
		case "cookie":
			cookieName := os.Getenv("GERRIT_COOKIE_NAME")
			cookieValue := os.Getenv("GERRIT_COOKIE_VALUE")
			if cookieName == "" || cookieValue == "" {
				logger.Fatalf("GERRIT_COOKIE_NAME and GERRIT_COOKIE_VALUE must be set for cookie authentication")
			}
			logger.Infof("Authentication mode: %s with cookie %s", authMode, cookieName)
			gerritClient.Authentication.SetCookieAuth(cookieName, cookieValue)
		case "basic":
			username := os.Getenv("GERRIT_USERNAME")
			password := os.Getenv("GERRIT_PASSWORD")
			if username == "" || password == "" {
				logger.Fatalf("GERRIT_USERNAME and GERRIT_PASSWORD must be set for basic authentication")
			}
			logger.Infof("Authentication mode: %s with user %s", authMode, username)
			gerritClient.Authentication.SetBasicAuth(username, password)
		case "digest":
			username := os.Getenv("GERRIT_USERNAME")
			password := os.Getenv("GERRIT_PASSWORD")
			if username == "" || password == "" {
				logger.Fatalf("GERRIT_USERNAME and GERRIT_PASSWORD must be set for digest authentication")
			}
			logger.Infof("Authentication mode: %s with user %s", authMode, username)
			gerritClient.Authentication.SetDigestAuth(username, password)
		default:
			logger.Infof("No authentication mode specified, using anonymous access")
	}
	if err != nil {
		logger.Fatalf("Failed to create Gerrit client: %v", err)
	}
	mcpServer := mcp.NewServer(mcp.WithGerritClient(gerritClient))
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- mcpServer.ServeSSE(host)
	}()

	// Wait for signal or error
	select {
	case err := <-errChan:
		if err != nil {
			logger.Fatalf("Server error: %v", err)
		}
	case sig := <-sigChan:
		logger.Infof("Received signal: %v", sig)
	}
	return
}
