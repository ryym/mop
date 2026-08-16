VERSION ?= dev
LDFLAGS := -X github.com/ryym/mop/internal/version.Version=$(VERSION)

.PHONY: build web go check-web test test-go test-web fmt clean

# The frontend bundle must exist before the Go build embeds it, so `go build`
# alone is never enough. That is also why `go install` is unsupported.
build: web go

web: node_modules
	bun run build

node_modules: package.json bun.lock
	bun install
	@touch node_modules

# web/dist is embedded with `all:dist`, which matches even an empty directory:
# without this check `go build` happily produces a binary whose preview page
# has no JS at all.
check-web:
	@test -f web/dist/mop.js || { \
		echo "web/dist/mop.js is missing. Run 'make web' (or 'make build')."; \
		exit 1; \
	}

go: check-web
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
	find web/dist -type f ! -name .gitkeep -delete
