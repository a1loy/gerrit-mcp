# Gerrit MCP Server

An MCP (Model Context Protocol) server that provides access to the choosen Gerrit instance.

This project implements an SSE-based MCP server that provides tools for:

1. Quering particular change/review by its number or id
2. Quering changes for specified project according filters
3. Quering available project

## Usage

1) Run on localhost:8080 in debug mode:

``DEBUG=true ./gerrit-mcp -port 8080 -addr 127.0.0.1``

2) Run on localhost:8080 with basic auth credentials for gerrit:

``GERRIT_USERNAME=john GERRIT_PASSWORD=johnP@ssword ./gerrit-mcp -port 8080 -addr 127.0.0.1 -with-auth=basic``
