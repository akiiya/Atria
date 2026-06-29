SHELL := /bin/bash
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
ifeq ($(strip $(VERSION)),)
VERSION := dev
endif
LDFLAGS := -s -w -X github.com/user/atria/internal/version.Version=$(VERSION)

.PHONY: build test vet release clean

build:
	@mkdir -p dist
	npm --prefix frontend install
	npm --prefix frontend run build
	touch web/static/dist/.gitkeep
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/atria ./cmd/atria
	@echo "built dist/atria (version=$(VERSION))"

test:
	go test ./...

vet:
	go vet ./...

release:
	bash ./scripts/release.sh

clean:
	rm -rf dist tmp
