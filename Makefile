.PHONY: test vet fmt-check check run

test:
	go test ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need gofmt:"; gofmt -l .; exit 1)

check: fmt-check vet test

run:
	go run ./cmd/storverity list
