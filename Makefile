.PHONY: chatid fmt fmt-check vet test test-race build

chatid:
	./scripts/telegram-chat-id.sh

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
