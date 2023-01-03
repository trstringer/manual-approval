IMAGE_REPO=ghcr.io/trstringer/manual-approval

# Great for building a single architecture's image
.PHONY: build
build:
	@if [ -z "$$VERSION" ]; then \
		echo "VERSION is required"; \
		exit 1; \
	fi
	docker build -t $(IMAGE_REPO):$$VERSION .

.PHONY: push
push:
	@if [ -z "$$VERSION" ]; then \
		echo "VERSION is required"; \
		exit 1; \
	fi
	docker push $(IMAGE_REPO):$$VERSION

.PHONY: test
test:
	go test -v .

.PHONY: lint
lint:
	docker run --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.46.2 golangci-lint run -v

# Builds multiple architectures at once.
# Requires docker buildx and QEMU to be configured.
# Because of how docker buildx works, in that it _must_ push when it builds, push is the default of this task.
.PHONY: buildx
buildx:
	@if [ -z "$$VERSION" ]; then \
		echo "VERSION is required"; \
		exit 1; \
	fi
	docker buildx build -t $(IMAGE_REPO):$$VERSION --platform linux/amd64,linux/arm64 --push .
