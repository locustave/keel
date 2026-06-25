BINARY  = keel
CMD     = ./cmd/keel
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -s -w -X main.version=$(VERSION)

.PHONY: build test release clean install

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

test:
	go test ./...

release:
	mkdir -p dist
	GOOS=darwin  GOARCH=arm64  go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  $(CMD)
	GOOS=darwin  GOARCH=amd64  go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  $(CMD)
	GOOS=linux   GOARCH=amd64  go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   $(CMD)
	GOOS=linux   GOARCH=arm64  go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64   $(CMD)

install: build
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -rf bin/ dist/
