.PHONY: build test lint clean docker-build

build:
	go build -o bin/gerrit-mcp cmd/server/main.go

test:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

docker-build:
	docker build -t gerrit-mcp .