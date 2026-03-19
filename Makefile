.PHONY: build setup-tools install test clean

# Build the slyds binary
build:
	go build -o slyds .

# Install required Go tools
setup-tools:
	go install github.com/spf13/cobra-cli@latest

# Install slyds into $GOBIN
install:
	go build -ldflags="$(LDFLAGS)" -o ${GOBIN}/slyds .

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f slyds

resymlink:
	mkdir -p locallinks
	rm -Rf locallinks/*
	cd locallinks && ln -s ~/newstack
