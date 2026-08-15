VERSION ?= dev
LDFLAGS := -X github.com/ryym/mop/internal/version.Version=$(VERSION)

.PHONY: build web go test test-go test-web fmt clean

# The frontend bundle must exist before the Go build embeds it, so `go build`
# alone is never enough. That is also why `go install` is unsupported.
build: web go

web: node_modules
	bun run build

node_modules: package.json bun.lock
	bun install
	@touch node_modules

go:
	go build -ldflags "$(LDFLAGS)" -o mop ./cmd/mop

test: test-go test-web

test-go:
	go test ./...

test-web: node_modules
	bun test

fmt:
	gofmt -w .

clean:
	rm -f mop
	rm -f web/dist/mop.js web/dist/mop.css
