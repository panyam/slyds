.PHONY: build setup-tools install test clean version

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/panyam/slyds/cmd.Version=$(VERSION)

# Build the slyds binary
build:
	go build -ldflags="$(LDFLAGS)" -o slyds .

# Install required Go tools
setup-tools:
	go install github.com/spf13/cobra-cli@latest

# Install slyds into $GOBIN
install:
	go build -ldflags="$(LDFLAGS)" -o ${GOBIN}/slyds .

# Run tests
test:
	go test ./...

# Print the version that would be injected
version:
	@echo $(VERSION)

# Clean build artifacts
clean:
	rm -f slyds

resymlink:
	mkdir -p locallinks
	rm -Rf locallinks/*
	cd locallinks && ln -s ~/newstack
