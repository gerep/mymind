BINARY = mymind
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build release-linux clean

build:
	go build -o $(BINARY) .

release-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY)-linux-arm64 .

clean:
	rm -f $(BINARY) $(BINARY)-linux-*
