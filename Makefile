.PHONY: all test check fmt fmtcheck vet staticcheck run-tests open_coverage clean

# Default target. gofmt + vet + staticcheck + unit tests with 100% coverage
# (cached, no race). Use `make test` for the strict pass with -race -shuffle=on.
all: run-tests
	@echo "✓ all green"

# Strict test pass: clean cache, race detector, shuffled. This is what CI runs.
test:
	@go clean -testcache
	@GOGC=off $(MAKE) run-tests TEST_FLAGS="-race -shuffle=on"

# Static gates (gofmt + go vet + staticcheck if installed).
check: fmtcheck vet staticcheck

fmt:
	@gofmt -w .

fmtcheck:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "ERROR: gofmt offenders (run 'make fmt'):"; echo "$$out"; \
		exit 1; \
	fi; \
	echo "✓ gofmt clean"

vet:
	@go vet ./...
	@echo "✓ go vet clean"

# staticcheck is optional. Install with:
#   go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck:
	@if ! command -v staticcheck >/dev/null 2>&1; then \
		echo "(staticcheck not installed — skipping)"; \
		exit 0; \
	fi; \
	out="$$(staticcheck ./... 2>&1 | grep -v 'file requires newer Go version' || true)"; \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		echo "ERROR: staticcheck reported findings"; \
		exit 1; \
	fi; \
	echo "✓ staticcheck clean"

# Run unit tests with the 100% coverage gate (excluding patterns in .covignore).
# Usage: make run-tests TEST_FLAGS="-race -shuffle=on"
run-tests: check
	@set -e; \
	tmpfile=$$(mktemp); patfile=$$(mktemp); \
	trap 'rm -f $$tmpfile $$patfile' EXIT; \
	if ! go test -cover $(TEST_FLAGS) ./... -coverprofile=coverage.tmp.out > $$tmpfile 2>&1; then \
		cat $$tmpfile; exit 1; \
	fi; \
	grep -v -E '^[[:space:]]*(#|$$)' .covignore > $$patfile || true; \
	if [ -s $$patfile ]; then \
		grep -v -E -f $$patfile coverage.tmp.out > coverage.out; \
	else \
		cp coverage.tmp.out coverage.out; \
	fi; \
	if go tool cover -func=coverage.out | tail -1 | grep -v '100.0%'; then \
		echo "ERROR: coverage is not 100% — see coverage.out (make open_coverage)"; \
		go tool cover -func=coverage.out | grep -v '100.0%' || true; \
		exit 1; \
	fi; \
	echo "✓ coverage 100% (excluding .covignore)"

open_coverage:
	go tool cover -html=coverage.out

clean:
	rm -f coverage.out coverage.tmp.out
