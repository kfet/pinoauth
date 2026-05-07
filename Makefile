.PHONY: all default test test-fast run-tests check fmt fmtcheck vet open_coverage clean

# Default target: gofmt + vet + unit tests with 100% coverage gate (cached,
# no race). Use `make test` for the strict pass with -race -shuffle=on.
default: test-fast
	@echo "✓ all green"

all: default

# check runs the static gates (gofmt + go vet).
check: fmtcheck vet

# fmt rewrites all Go files to canonical gofmt style.
fmt:
	@gofmt -w .

# fmtcheck fails if any Go file is not gofmt-clean.
fmtcheck:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "ERROR: gofmt offenders (run 'make fmt'):"; \
		echo "$$out"; \
		exit 1; \
	fi
	@echo "✓ gofmt clean"

vet:
	@go vet ./...
	@echo "✓ go vet clean"

# Run unit tests with 100% coverage gate (excluding paths in .covignore).
# Usage: make run-tests TEST_FLAGS="-race -shuffle=on"
run-tests: check
	@tmpfile=$$(mktemp); \
	trap 'rm -f $$tmpfile' EXIT; \
	go test -cover $(TEST_FLAGS) ./... -coverprofile=coverage.tmp.out > $$tmpfile 2>&1; \
	if [ $$? -ne 0 ]; then \
		cat $$tmpfile; \
		exit 1; \
	fi
	@grep -v -E -f .covignore coverage.tmp.out > coverage.out
	@if go tool cover -func=coverage.out | tail -1 | grep -v '100.0%'; then \
		echo "ERROR: coverage is not 100% — see coverage.out (make open_coverage)"; \
		go tool cover -func=coverage.out | grep -v '100.0%' || true; \
		exit 1; \
	fi
	@echo "✓ coverage 100% (excluding .covignore)"

# Strict test pass: clean cache, race detector, shuffled. This is what CI runs.
test:
	@go clean -testcache
	@GOGC=off $(MAKE) run-tests TEST_FLAGS="-race -shuffle=on"

# Fast test pass: cached, no race.
test-fast:
	@$(MAKE) run-tests

open_coverage:
	go tool cover -html=coverage.out

clean:
	rm -f coverage.out coverage.tmp.out
