.PHONY: all test check fmt fmtcheck vet staticcheck _staticcheck run-tests open_coverage clean

# Quiet runner: $(call RUN,label,cmd) — runs cmd silently, prints "✓ label" on
# success, dumps captured output and exits non-zero on failure. Set V=1 for
# verbose output.
ifdef V
  define RUN
	@echo "→ $(1)"
	@$(2)
  endef
else
  define RUN
	@_log=$$(mktemp); \
	if ( $(2) ) > $$_log 2>&1; then \
		echo "✓ $(1)"; rm -f $$_log; \
	else \
		rc=$$?; cat $$_log; rm -f $$_log; exit $$rc; \
	fi
  endef
endif

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
	$(call RUN,gofmt clean,out=$$(gofmt -l .); test -z "$$out" || { echo "gofmt offenders (run 'make fmt'):"; echo "$$out"; exit 1; })

vet:
	$(call RUN,go vet clean,go vet ./...)

# staticcheck is optional. Install with:
#   go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck:
	@if ! command -v staticcheck >/dev/null 2>&1; then \
		echo "(staticcheck not installed — skipping)"; exit 0; \
	fi; \
	$(MAKE) --no-print-directory _staticcheck

_staticcheck:
	$(call RUN,staticcheck clean,out=$$(staticcheck ./... 2>&1 | grep -v 'file requires newer Go version' || true); test -z "$$out" || { echo "$$out"; exit 1; })

# Run unit tests with the 100% coverage gate (excluding patterns in .covignore).
# Usage: make run-tests TEST_FLAGS="-race -shuffle=on"
run-tests: check
	$(call RUN,tests pass,go test -cover $(TEST_FLAGS) ./... -coverprofile=coverage.tmp.out)
	$(call RUN,coverage clean,go run github.com/kfet/covgate/cmd/covgate@v0.1.0 -profile=coverage.tmp.out -out=coverage.out -ignore=.covignore -min=100)
	@rm -f coverage.tmp.out

open_coverage:
	go tool cover -html=coverage.out

clean:
	rm -f coverage.out coverage.tmp.out
