.PHONY: build setup-tools install test clean version examples examples-serve gh-pages

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

EXAMPLES := examples/slyds-intro examples/rich-content examples/hacker-showcase

# Build all example presentations into examples/dist/
examples: build
	@mkdir -p examples/dist/examples
	@for dir in $(EXAMPLES); do \
		./slyds build "$$dir"; \
		name=$$(basename "$$dir"); \
		mkdir -p "examples/dist/examples/$$name"; \
		cp "$$dir/dist/index.html" "examples/dist/examples/$$name/"; \
	done
	@cp examples/landing/index.html examples/dist/
	@echo "Examples built to examples/dist/"

# Serve built examples locally
examples-serve: examples
	@echo "Serving examples at http://localhost:8080"
	@cd examples/dist && python3 -m http.server 8080

# Deploy examples to gh-pages branch
gh-pages: examples
	@echo "Deploying to gh-pages branch..."
	@if [ -d examples/dist/.git ]; then \
		echo "Error: examples/dist appears to be a git repo. Please remove it first."; \
		exit 1; \
	fi
	@cd examples/dist && \
		touch .nojekyll && \
		git init && \
		git add -A && \
		git commit -m "Deploy examples to GitHub Pages" && \
		git branch -M gh-pages && \
		git remote add origin git@panyam-github:panyam/slyds.git && \
		git push -f origin gh-pages
	@echo "Deployed! Enable GitHub Pages in repo settings to serve from gh-pages branch."
	@rm -rf examples/dist/.git
