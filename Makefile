.PHONY: build test vet check

build:
	mkdir -p bin
	go build -o bin/vibecodemap ./cmd/vibecodemap

test:
	go test ./...

vet:
	go vet ./...

check: test vet
