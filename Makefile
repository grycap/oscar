.PHONY: help build push rollout deploy

REGISTRY ?= localhost:5001
IMAGE_NAME ?= oscar
IMAGE_TAG ?= devel
NAMESPACE ?= oscar
DEPLOYMENT ?= $(IMAGE_NAME)
BUILD_CONTEXT ?= ../oscar
KUBECTL ?= kubectl

IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
KUBE_CONTEXT_ARG := $(if $(KUBE_CONTEXT),--context $(KUBE_CONTEXT),)

help:
	@echo "Available targets:"
	@echo "  build    - Build Docker image $(IMAGE)"
	@echo "  push     - Push image $(IMAGE) to registry"
	@echo "  rollout  - Restart Kubernetes deployment $(DEPLOYMENT) in namespace $(NAMESPACE)"
	@echo "  deploy   - Build, push, and rollout (default pipeline)"
	@echo ""
	@echo "Optional variables:"
	@echo "  KUBE_CONTEXT - Kubernetes context to use for rollout"

build:
	docker build -t $(IMAGE) $(BUILD_CONTEXT)

push: build
	docker push $(IMAGE)

rollout:
	$(KUBECTL) $(KUBE_CONTEXT_ARG) rollout restart deployment/$(DEPLOYMENT) -n $(NAMESPACE)

deploy: push rollout
