# Makefile for STF (Simple Transfer)

# Docker settings
DOCKER_USERNAME := kiyor
IMAGE_NAME := stf
IMAGE_TAG := latest
FULL_IMAGE := $(DOCKER_USERNAME)/$(IMAGE_NAME):$(IMAGE_TAG)

# Build platforms
PLATFORMS := linux/amd64,linux/arm64,linux/arm/v7

# Go build settings
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
BINARY_NAME := stf

# Build flags
LDFLAGS := -w -s
BUILD_FLAGS := -a -installsuffix cgo -ldflags="$(LDFLAGS)"

.PHONY: all build clean test test-e2e vet image image-load image-local push help

all: build

## build: Build binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 $(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	@echo "Clean complete"

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-e2e: Run end-to-end tests on Docker image
test-e2e:
	@echo "Running E2E tests..."
	@bash test_e2e.sh

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## image: Build multi-platform Docker image (cached, not loaded)
image:
	@echo "Building multi-platform Docker image..."
	@docker buildx create --use --name stf-builder 2>/dev/null || docker buildx use stf-builder
	docker buildx build \
		--platform $(PLATFORMS) \
		-t $(FULL_IMAGE) \
		.
	@echo "Multi-platform build complete: $(PLATFORMS)"
	@echo "Note: Multi-platform images are cached but not loaded to local Docker."
	@echo "      Use 'make image-load' to build and load current platform only."

## image-load: Build and load Docker image for current platform
image-load:
	@echo "Building and loading Docker image for current platform..."
	docker buildx build \
		-t $(FULL_IMAGE) \
		--load \
		.
	@echo "Image built and loaded: $(FULL_IMAGE)"

## image-local: Build and load for specific platform (usage: make image-local PLATFORM=linux/arm64)
image-local:
	@PLATFORM=$${PLATFORM:-linux/amd64}; \
	echo "Building Docker image for $$PLATFORM..."; \
	docker buildx build \
		--platform $$PLATFORM \
		-t $(FULL_IMAGE) \
		--load \
		.; \
	echo "Image built and loaded: $(FULL_IMAGE) ($$PLATFORM)"

## push: Build and push multi-platform image to DockerHub
push:
	@echo "Logging into DockerHub..."
	@docker buildx create --use --name stf-builder 2>/dev/null || docker buildx use stf-builder
	@echo "Building and pushing multi-platform image to DockerHub..."
	docker buildx build \
		--platform $(PLATFORMS) \
		-t $(FULL_IMAGE) \
		--push \
		.
	@echo "Successfully pushed $(FULL_IMAGE)"
	@echo "Supported platforms: $(PLATFORMS)"

## tag: Tag image with additional version
tag:
	@read -p "Enter version tag (e.g., v1.0.0): " VERSION; \
	docker tag $(FULL_IMAGE) $(DOCKER_USERNAME)/$(IMAGE_NAME):$$VERSION && \
	echo "Tagged as $(DOCKER_USERNAME)/$(IMAGE_NAME):$$VERSION"

## run: Run the binary locally
run: build
	./$(BINARY_NAME)

## docker-run: Run the Docker image locally
docker-run:
	docker run --rm -p 30000:30000 $(FULL_IMAGE)

## help: Show this help message
help:
	@echo "STF Makefile Commands:"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
