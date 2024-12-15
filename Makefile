
all:
	make build
	make test
	make lint

build:
	go mod tidy
	go build -o bin/lid cmd/lid/main.go

test:
	go test -v ./...

.PHONY: lint
lint:
	gofmt -w .
