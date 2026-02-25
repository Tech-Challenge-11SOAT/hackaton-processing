.PHONY: test test-race coverage fmt tidy

test:
	go test ./...

test-race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt:
	go fmt ./...

tidy:
	go mod tidy
