.PHONY: build hu test vet fmt

build:
	go build ./...

hu:
	go build -o bin/hu ./cmd/hu

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .
