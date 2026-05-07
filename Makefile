.PHONY: all test vet cover clean

all: vet test

test:
	go test -race ./...

vet:
	go vet ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -f coverage.out coverage.tmp.out
