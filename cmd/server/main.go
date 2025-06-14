package main

import (
	"flag"
	"fmt"
	"gerrit-mcp/internal/logger"
	"gerrit-mcp/internal/gerrit"
	"gerrit-mcp/pkg/mcp"
	"os"
	"os/signal"
	"syscall"
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
	flag.Parse()
	host := fmt.Sprintf("%s:%s", *addr, *port)
	logger.Debugf("Starting Gerrit MCP server on %s", host)
	logger.Debugf("Gerrit instance: %s", *gerritInstance)

	gerritClient := gerrit.NewClient(*gerritInstance)
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
