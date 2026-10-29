.PHONY: build test run deps check-deps

build:
	go build -o nmea-server ./cmd/nmea-server

test:
	go test ./...

# replay the bundled recording: http://localhost:8080
run:
	go run ./cmd/nmea-server -source testdata/sample.nmea

# web/ui/node_modules is committed so the Go module embeds it (no build step):
# run `make deps` and commit the result; check-deps fails when the
# committed files differ from the lockfile.
deps:
	npm install --prefix web/ui --no-audit --no-fund

check-deps:
	npm ci --prefix web/ui --no-audit --no-fund
	git diff --exit-code --stat -- web/ui/node_modules
	@test -z "$$(git status --porcelain -- web/ui/node_modules)" || (git status --short -- web/ui/node_modules; exit 1)
