GO ?= go
BIN ?= ctx-clip
BINDIR ?= $(HOME)/.local/bin
GOCACHE ?= $(HOME)/.cache/go-build
GOMODCACHE ?= $(HOME)/go/pkg/mod
GOENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: fmt vet test test-race build install clean check

fmt:
	gofmt -w $(shell find . -type f -name '*.go' -not -path './.git/*')

vet:
	$(GOENV) $(GO) vet ./...

test:
	$(GOENV) $(GO) test ./...

test-race:
	$(GOENV) $(GO) test -race ./...

build:
	$(GOENV) $(GO) build -o $(BIN) .

install: build
	mkdir -p $(BINDIR)
	install -m 0755 $(BIN) $(BINDIR)/$(BIN)

clean:
	rm -f $(BIN)

check: fmt vet test test-race build
