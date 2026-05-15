BINARY := gitcrawl
VERSION ?= dev

.PHONY: build generate-sqlc test test-coverage run clean

build:
	mkdir -p bin
	go build -ldflags "-X github.com/openclaw/gitcrawl/internal/cli.version=$(VERSION)" -o bin/$(BINARY) ./cmd/gitcrawl

generate-sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

test:
	go test ./...

test-coverage:
	go test ./... -covermode=atomic -coverprofile=coverage.out
	@total="$$(go tool cover -func=coverage.out | awk '/^total:/ { sub(/%/, "", $$3); print $$3 }')"; \
	echo "total coverage: $${total}%"; \
	awk -v total="$$total" 'BEGIN { if (total + 0 < 85.0) { printf("coverage %.1f%% is below 85.0%%\n", total); exit 1 } }'

run:
	go run ./cmd/gitcrawl $(ARGS)

clean:
	rm -rf bin
