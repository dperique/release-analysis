# The binary to build (just the basename).
BINARY_NAME=release-analysis

# The Go compiler to use
GO=go

# Build the project
build:
	$(GO) build -o $(BINARY_NAME) main.go

# Clean up the project
clean:
	rm -f $(BINARY_NAME)

# Lint the project
# Install like this: GO111MODULE=off go get -u golang.org/x/lint/golint
lint:
	golint ./...

# Phony targets to avoid conflict with files of the same name and to improve performance
.PHONY: build clean lint