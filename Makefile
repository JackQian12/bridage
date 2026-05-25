.PHONY: build test lint vuln

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath ./...

test:
	go test ./...

lint:
	go vet ./...

vuln:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...
