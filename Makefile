.PHONY: all build test clean docker-build deploy kind-setup swagger

GO_BIN ?= /usr/local/go/bin/go
MODULES = mq streamer collector api-gateway

all: build test swagger

build:
	@for dir in $(MODULES); do \
		echo "Building $$dir..."; \
		cd $$dir && $(GO_BIN) build ./... && cd ..; \
	done

test:
	@for dir in $(MODULES); do \
		echo "Testing $$dir..."; \
		cd $$dir && $(GO_BIN) test -v ./... && cd ..; \
	done

clean:
	@for dir in $(MODULES); do \
		echo "Cleaning $$dir..."; \
		cd $$dir && $(GO_BIN) clean && cd ..; \
	done

swagger:
	@echo "Generating Swagger docs..."
	@cd api-gateway && $(GO_BIN) run github.com/swaggo/swag/cmd/swag init -g cmd/main.go

docker-build:
	@for dir in $(MODULES); do \
		echo "Building Docker image for $$dir..."; \
		docker build -t telemetry-$$dir:latest ./$$dir; \
	done

kind-setup:
	@echo "Setting up KIND cluster..."
	kind create cluster --name telemetry-pipeline
	kubectl cluster-info --context kind-telemetry-pipeline

deploy:
	@echo "Deploying to KIND..."
	helm install telemetry-pipeline ./deploy/helm

coverage:
	@for dir in $(MODULES); do \
		echo "Generating coverage for $$dir..."; \
		cd $$dir && $(GO_BIN) test -coverprofile=coverage.out ./... && cd ..; \
	done
