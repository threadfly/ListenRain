export GO111MODULE=on

all:
GO := go
PKG := github.com/threadfly/ListenRain/example/benchmark
VERSION := $(shell git describe --tags | sed 's/^v//g')
COMMIT_ID := $(shell git rev-parse --short HEAD)
GO_VERSION := $(shell $(GO) version|sed 's/go version[[:blank:]]//g')
BUILD_TIME := $(shell date +%Y-%m-%d:%H:%M:%S)
LDFLAGS = -s -w

ENV := $(shell $(GO) env)

env:
	$(warning $(ENV))

linux: env
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -x -ldflags "${LDFLAGS}" 

macos: env
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -v -x -ldflags "${LDFLAGS}" 

clean:
	rm -rf benchmark
