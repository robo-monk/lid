
all:
	make build
	make test
	make lint

build:
	go mod tidy
	go build -o bin/lid cmd/lid/main.go


.PHONY: lint test

test:
	go clean -testcache
	go test -v ./...

lint:
	gofmt -w .
