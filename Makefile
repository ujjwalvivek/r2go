.PHONY: run build test vet fmt check clean

run:
	go run ./cmd/r2go

build:
	mkdir -p dist
	go build -o dist/r2go ./cmd/r2go

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find cmd internal -name '*.go')

check: test vet

clean:
	rm -rf dist coverage.out
