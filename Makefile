.PHONY: fmt fmt-check vet test test-race build

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	test -z "$(gofmt -l ./cmd ./internal)"

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

build:
	go build ./...
