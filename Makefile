BINARY  := nogo
VERSION ?= dev

.PHONY: all build install tidy clean

all: build

## Download dependencies and build
build: tidy
	go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY) .

## Install nogo to $GOPATH/bin (or ~/go/bin)
install: tidy
	go install -ldflags="-X main.version=$(VERSION)" .

## Tidy go.sum
tidy:
	go mod tidy

## Remove build artifacts
clean:
	rm -f $(BINARY)
