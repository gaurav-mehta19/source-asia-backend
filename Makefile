.PHONY: run build test fmt vet tidy clean

BINARY := bin/server
MAIN   := ./cmd/server

run:
	go run $(MAIN)

build:
	go build -o $(BINARY) $(MAIN)

test:
	go test -race -count=1 ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
