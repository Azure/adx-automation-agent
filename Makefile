SRC = $(shell find . -name '*.go')
REV = $(shell git rev-parse --verify HEAD)

.PHONY: all
all: mod test fmt lint vet a01dispatcher a01droid

.PHONY: test
test: ${SRC}
	go test ./...

.PHONY: fmt
fmt: ${SRC}
	go fmt ./...

.PHONY: lint
lint: ${SRC}
	golint ./...

.PHONY: vet
vet: ${SRC}
	go vet ./...

a01dispatcher: $(shell find ./agents/dispatcher -name '*.go') $(shell find ./sdk -name '*.go')
	go build -o a01dispatcher -ldflags "-X main.version=${TRAVIS_TAG} -X main.sourceCommit=${REV}" ./agents/dispatcher

a01droid: $(shell find ./agents/droid -name '*.go') $(shell find ./sdk -name '*.go')
	go build -o a01droid -ldflags "-X main.version=${TRAVIS_TAG} -X main.sourceCommit=${REV}" ./agents/droid

.PHONY: clean
clean:
	rm -f a01dispatcher a01droid
	go clean -modcache

.PHONY: mod
mod: go.sum
	go mod download

go.sum: go.mod
	go mod tidy
