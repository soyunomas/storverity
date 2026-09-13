.PHONY: test vet fmt-check check run frontend-test frontend-check frontend-build desktop-build

test:
	go test -race ./cmd/... ./internal/...

vet:
	go vet ./cmd/... ./internal/...

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need gofmt:"; gofmt -l .; exit 1)

check: fmt-check vet test

run:
	go run ./cmd/storverity list

frontend-test:
	cd frontend && npm test

frontend-check:
	cd frontend && npm run check

frontend-build:
	cd frontend && npm run build

desktop-build:
	wails build -clean -tags webkit2_41
