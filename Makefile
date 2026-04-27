.PHONY: help build push rollout deploy

REGISTRY ?= localhost:5001
IMAGE_NAME ?= oscar
IMAGE_TAG ?= devel
NAMESPACE ?= oscar
DEPLOYMENT ?= $(IMAGE_NAME)
BUILD_CONTEXT ?= ../oscar
KUBECTL ?= kubectl
DOCKER ?= docker
DOCKER_BUILDKIT ?= 1
BUILDKIT_PROGRESS ?= auto
NO_CACHE ?=

IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
KUBE_CONTEXT_ARG := $(if $(KUBE_CONTEXT),--context $(KUBE_CONTEXT),)
DOCKER_BUILD_ENV := DOCKER_BUILDKIT=$(DOCKER_BUILDKIT) \
	BUILDKIT_PROGRESS=$(BUILDKIT_PROGRESS)
DOCKER_BUILD_FLAGS := $(if $(NO_CACHE),--no-cache,)

help:
	@echo "Available targets:"
	@echo "  build    - Build Docker image $(IMAGE)"
	@echo "  push     - Push image $(IMAGE) to registry"
	@echo "  rollout  - Restart Kubernetes deployment $(DEPLOYMENT) in namespace $(NAMESPACE)"
	@echo "  deploy   - Build, push, and rollout (default pipeline)"
	@echo ""
	@echo "Optional variables:"
	@echo "  KUBE_CONTEXT - Kubernetes context to use for rollout"
	@echo "  DOCKER_BUILDKIT - Enable Docker BuildKit (default: 1)"
	@echo "  BUILDKIT_PROGRESS - BuildKit progress output (default: auto)"
	@echo "  NO_CACHE - Set to 1 to force docker build --no-cache"

build:
	$(DOCKER_BUILD_ENV) $(DOCKER) build $(DOCKER_BUILD_FLAGS) \
		-t $(IMAGE) $(BUILD_CONTEXT)

push: build
	$(DOCKER) push $(IMAGE)

rollout:
	$(KUBECTL) $(KUBE_CONTEXT_ARG) rollout restart deployment/$(DEPLOYMENT) -n $(NAMESPACE)

deploy: push rollout
