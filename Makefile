.PHONY: build lint vet fmt clean

build:
	CGO_ENABLED=1 go build -o bin/tts-api ./cmd/server/

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -l -s -w .

clean:
	rm -rf bin/
