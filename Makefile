.PHONY: build build-all clean test

VERSION ?= dev

build:
	go build -ldflags "-X github.com/Binmave/binmave-cli/internal/commands.Version=$(VERSION)" -o binmave ./cmd/binmave

build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/Binmave/binmave-cli/internal/commands.Version=$(VERSION)" -o binmave-linux-amd64 ./cmd/binmave
	GOOS=linux GOARCH=arm64 go build -ldflags "-X github.com/Binmave/binmave-cli/internal/commands.Version=$(VERSION)" -o binmave-linux-arm64 ./cmd/binmave
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X github.com/Binmave/binmave-cli/internal/commands.Version=$(VERSION)" -o binmave-darwin-amd64 ./cmd/binmave
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X github.com/Binmave/binmave-cli/internal/commands.Version=$(VERSION)" -o binmave-darwin-arm64 ./cmd/binmave
	GOOS=windows GOARCH=amd64 go build -ldflags "-X github.com/Binmave/binmave-cli/internal/commands.Version=$(VERSION)" -o binmave-windows-amd64.exe ./cmd/binmave

test:
	go test ./...

clean:
	rm -f binmave binmave-*

install:
	go install -ldflags "-X github.com/Binmave/binmave-cli/internal/commands.Version=$(VERSION)" ./cmd/binmave
