default:
    @just --list

build:
    go build ./...

test:
    go test ./...

vet:
    go vet ./...

check: build test vet

# Build a standalone binary for local session launchers.
session-binary:
    go build -o /tmp/obs-cli-session .
