.PHONY: test test-race test-verbose

test:
	go test ./...

test-race:
	go test -race ./...

test-race-verbose:
	go test -v -race ./...
