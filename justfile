version := trim(read("VERSION"))

default:
    @just --list

build:
    go build -ldflags "-X main.version={{version}}" ./...

test:
    go test ./...

vet:
    go vet ./...

check: build test vet

# Build a standalone binary for local session launchers.
session-binary:
    go build -ldflags "-X main.version={{version}}" -o /tmp/obs-cli-session .

binary:
    mkdir -p dist
    go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/obs-cli .

install:
    go install -trimpath -ldflags "-s -w -X main.version={{version}}" .

# The app embeds both binaries; there are no home-directory or PATH dependencies.
macos-app: binary
    ./integrations/macos/build.sh

install-macos-app: binary
    ./integrations/macos/build.sh --output "$HOME/Applications/OBS Recording.app" --force

macos-app-test: binary
    ./integrations/macos/build_test.sh

release-check: check
    goreleaser check
