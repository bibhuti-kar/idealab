.PHONY: dev test lint format build clean install-tools preinstall

# Development
dev:
	docker compose up --build

dev-local:
	go run ./cmd/operator

# Testing
test:
	go test ./... -v -race -coverprofile=coverage.txt -covermode=atomic

test-short:
	go test ./... -short -v

coverage:
	go test ./... -coverprofile=coverage.txt -covermode=atomic
	go tool cover -html=coverage.txt -o coverage.html

# Code quality
lint:
	golangci-lint run ./...

format:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

# Build
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/operator ./cmd/operator
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/preinstall ./cmd/preinstall

build-docker:
	docker build -t idealab-operator:latest .

# Pre-install (run on host, not in container)
preinstall:
	sudo go run ./cmd/preinstall

# Kubernetes
deploy:
	kubectl apply -f deploy/crds/
	kubectl apply -f deploy/operator/

undeploy:
	kubectl delete -f deploy/operator/ || true
	kubectl delete -f deploy/crds/ || true

# Clean
clean:
	docker compose down -v --remove-orphans
	rm -rf bin/ coverage.txt coverage.html

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
