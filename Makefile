PACKAGE := github.com/CarlosArguelloDev/okshield/internal/version
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

LDFLAGS := -X $(PACKAGE).Version=$(VERSION) -X $(PACKAGE).Commit=$(COMMIT) -X $(PACKAGE).Date=$(DATE) -s -w

.PHONY: all build build-arm64 test vet clean tag

all: test build

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/okshield ./cmd/okshield

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/okshield-linux-arm64 ./cmd/okshield

test:
	go test -v -count=1 ./...

vet:
	go vet ./...

tag:
	@if [ -z "$(TAG)" ]; then echo "Uso: make tag TAG=v0.1.0"; exit 1; fi
	git tag -a $(TAG) -m "Release $(TAG)"
	@echo "Tag $(TAG) creado localmente. Ejecuta 'git push origin $(TAG)' para subirlo."

clean:
	rm -rf bin/
