IMAGE_REPO=ghcr.io/trstringer/manual-approval
TARGET_PLATFORM=linux/amd64,linux/arm64,linux/arm/v8
GO_DOCKER_IMAGE=golang:1.24

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build
build:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required"; \
		exit 1; \
	fi
	docker build -t $(IMAGE_REPO):$(VERSION) .

.PHONY: build_push
build_push:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required"; \
		exit 1; \
	fi
	docker buildx create --use --name mybuilder
	docker buildx build --push --platform $(TARGET_PLATFORM) -t $(IMAGE_REPO):$(VERSION) .
	docker buildx rm mybuilder


.PHONY: test
test:
	go test -v .

.PHONY: test_docker
test_docker:
	docker run --rm -v $$(pwd):/app -w /app $(GO_DOCKER_IMAGE) sh -c "go mod tidy && go test -v ."

.PHONY: lint
lint:
	docker run --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v2.1.6 golangci-lint run -v
