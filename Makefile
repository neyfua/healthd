BINARY := healthd
IMAGE   ?= ghcr.io/neyfua/healthd:latest
VERSION ?= dev
LDFLAGS := -X github.com/neyfua/healthd/internal/server.BuildVersion=$(VERSION)

.PHONY: build test run docker docker-run clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test -v -race -shuffle=on ./...

run: build
	./bin/$(BINARY)

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

docker-run: docker
	docker run --rm -p 8080:8080 $(IMAGE)

clean:
	rm -rf bin