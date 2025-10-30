.PHONY: build docker-build docker-build-multiarch docker-push deploy clean test buildx-setup

# Variables
IMAGE_NAME ?= hailo-device-plugin
IMAGE_TAG ?= latest
REGISTRY ?= ghcr.io/snu-rtos
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
PLATFORMS ?= linux/amd64,linux/arm64

# Build the binary locally
build:
	go mod tidy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o hailo-device-plugin .

# Setup Docker Buildx for multi-arch builds
buildx-setup:
	@echo "Setting up Docker Buildx..."
	@docker buildx inspect hailo-builder > /dev/null 2>&1 || \
		docker buildx create --name hailo-builder --driver docker-container --bootstrap
	@docker buildx use hailo-builder
	@echo "Buildx builder 'hailo-builder' is ready"

# Build the Docker image (single architecture, local)
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(FULL_IMAGE_NAME)

# Build and push multi-architecture Docker image (amd64 + arm64)
docker-build-multiarch: buildx-setup
	@echo "Building and pushing multi-architecture image for $(PLATFORMS)..."
	@echo "Target: $(FULL_IMAGE_NAME)"
	docker buildx build \
		--platform $(PLATFORMS) \
		--tag $(FULL_IMAGE_NAME) \
		--push \
		.

# Push the Docker image to registry (single architecture)
docker-push: docker-build
	docker push $(FULL_IMAGE_NAME)

# Deploy DaemonSet to Kubernetes
deploy:
	kubectl apply -f deploy/hailo-device-plugin.yaml

# Deploy with custom image
deploy-custom:
	cat deploy/hailo-device-plugin.yaml | sed 's|image: hailo-device-plugin:latest|image: $(FULL_IMAGE_NAME)|' | kubectl apply -f -

# Undeploy from Kubernetes
undeploy:
	kubectl delete -f deploy/hailo-device-plugin.yaml --ignore-not-found=true

# Clean up built artifacts
clean:
	rm -f hailo-device-plugin
	go clean

# Run tests
test:
	go test -v ./...

# Check deployment status
status:
	@echo "=== DaemonSet Status ==="
	kubectl get ds -n kube-system hailo-device-plugin
	@echo "\n=== Pod Status ==="
	kubectl get pods -n kube-system -l app=hailo-device-plugin
	@echo "\n=== Pod Logs (if running) ==="
	kubectl logs -n kube-system -l app=hailo-device-plugin --tail=50 || true

# Check node capacity for Hailo devices
check-nodes:
	@echo "=== Node Hailo Resources ==="
	kubectl get nodes -o custom-columns=NAME:.metadata.name,HAILO:.status.capacity.hailo\\.ai/npu

# All-in-one: build, create docker image, and deploy
all: build docker-build deploy

# Help
help:
	@echo "Available targets:"
	@echo "  build                  - Build the binary locally"
	@echo "  buildx-setup           - Setup Docker Buildx for multi-arch builds"
	@echo "  docker-build           - Build the Docker image (single architecture)"
	@echo "  docker-build-multiarch - Build and push multi-arch image (amd64 + arm64)"
	@echo "  docker-push            - Push the Docker image to registry (single arch)"
	@echo "  deploy                 - Deploy to Kubernetes cluster"
	@echo "  deploy-custom          - Deploy with custom image from REGISTRY variable"
	@echo "  undeploy               - Remove from Kubernetes cluster"
	@echo "  clean                  - Clean up built artifacts"
	@echo "  test                   - Run tests"
	@echo "  status                 - Check deployment status"
	@echo "  check-nodes            - Check node capacity for Hailo devices"
	@echo "  all                    - Build, dockerize, and deploy"
	@echo ""
	@echo "Variables:"
	@echo "  IMAGE_NAME      - Docker image name (default: hailo-device-plugin)"
	@echo "  IMAGE_TAG       - Docker image tag (default: latest)"
	@echo "  REGISTRY        - Docker registry (default: ghcr.io/snu-rtos)"
	@echo "  PLATFORMS       - Target platforms (default: linux/amd64,linux/arm64)"
