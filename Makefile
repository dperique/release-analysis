# The binary to build (just the basename).
RELEASE_ANALYSIS=release-analysis
GCS_FINDER=gcs-finder
GCS_NODE_DOWNLOAD=gcs-node-download

# The Go compiler to use
GO=go

# Build the project
build: build-release-analysis build-gcs-finder build-gcs-node-download

build-release-analysis:
	@module=$$(grep "module " go.mod | awk '{print $$2}'); \
	$(GO) build -gcflags='-N -l' $$module/cmd/$(RELEASE_ANALYSIS)

build-gcs-finder:
	@module=$$(grep "module " go.mod | awk '{print $$2}'); \
	$(GO) build -gcflags='-N -l' $$module/cmd/$(GCS_FINDER)

build-gcs-node-download:
	@module=$$(grep "module " go.mod | awk '{print $$2}'); \
	$(GO) build -gcflags='-N -l' $$module/cmd/$(GCS_NODE_DOWNLOAD)

# Clean up the project
clean:
	rm -f $(RELEASE_ANALYSIS) $(GCS_FINDER) $(GCS_NODE_DOWNLOAD)

# Lint the project
# Install like this: GO111MODULE=off go get -u golang.org/x/lint/golint
lint:
	golint ./...

# Phony targets to avoid conflict with files of the same name and to improve performance
.PHONY: build build-release-analysis build-gcs-finder build-gcs-node-download clean lint
