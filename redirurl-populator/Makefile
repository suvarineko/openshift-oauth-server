BINARY  ?= redirurl-populator
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
IMAGE   ?= redirurl-populator:$(VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet fmt tidy run docker-build clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

run:
	go run .

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

clean:
	rm -rf bin
