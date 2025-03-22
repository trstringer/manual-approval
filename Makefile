IMAGE_REPO ?= ghcr.io/snskarora/manual-approval
TARGET_PLATFORM ?= linux/amd64

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build
build:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required"; \
		exit 1; \
	fi
	docker build --platform $(TARGET_PLATFORM) -t $(IMAGE_REPO):$$VERSION .

.PHONY: push
push:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required"; \
		exit 1; \
	fi
	@echo $(AUTH) | docker login ghcr.io -u snskArora --password-stdin
	docker push $(IMAGE_REPO):$(VERSION)

.PHONY: test
test:
	go test -v .

.PHONY: lint
lint:
	docker run --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.46.2 golangci-lint run -v
