GOBUILD=go build -ldflags "-s -w"
BINARY_DIR=bin

.PHONY: $(MAKECMDGOALS)

APPS=$(wildcard cmd/*)

all: test build

build: $(APPS)
	@mkdir -p $(BINARY_DIR)
	@for app in $^ ; do \
		GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/prometheus-mimic-$$(basename $$app)_linux_amd64 $$app/main.go ; \
		GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BINARY_DIR)/prometheus-mimic-$$(basename $$app)_linux_arm64 $$app/main.go ; \
	done

test: 
	go test -cover ./...

clean: 
	go clean
	rm -rf $(BINARY_DIR)
