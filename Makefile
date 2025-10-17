.PHONY: build docker-build docker-push deploy clean test

# Variables
IMAGE_NAME ?= hailo-device-plugin
IMAGE_TAG ?= latest
REGISTRY ?= ghcr.io/snu-rtos
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Build the binary locally
build:
	go mod tidy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o hailo-device-plugin .

# Build the Docker image
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(FULL_IMAGE_NAME)

# Push the Docker image to registry
docker-push: docker-build
	docker push $(FULL_IMAGE_NAME)

# Deploy RBAC and DaemonSet to Kubernetes
deploy:
	kubectl apply -f deploy/daemonset.yaml

# Deploy with custom image
deploy-custom:
	cat deploy/daemonset.yaml | sed 's|image: hailo-device-plugin:latest|image: $(FULL_IMAGE_NAME)|' | kubectl apply -f -

# Undeploy from Kubernetes
undeploy:
	kubectl delete -f deploy/daemonset.yaml --ignore-not-found=true

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
	@echo "  build           - Build the binary locally"
	@echo "  docker-build    - Build the Docker image"
	@echo "  docker-push     - Push the Docker image to registry"
	@echo "  deploy          - Deploy to Kubernetes cluster"
	@echo "  deploy-custom   - Deploy with custom image from REGISTRY variable"
	@echo "  undeploy        - Remove from Kubernetes cluster"
	@echo "  clean           - Clean up built artifacts"
	@echo "  test            - Run tests"
	@echo "  status          - Check deployment status"
	@echo "  check-nodes     - Check node capacity for Hailo devices"
	@echo "  all             - Build, dockerize, and deploy"
	@echo ""
	@echo "Variables:"
	@echo "  IMAGE_NAME      - Docker image name (default: hailo-device-plugin)"
	@echo "  IMAGE_TAG       - Docker image tag (default: latest)"
	@echo "  REGISTRY        - Docker registry (default: docker.io/yourregistry)"
