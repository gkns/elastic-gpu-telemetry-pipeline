BINDIR   := bin
CMDS     := mq streamer collector api-gateway
REGISTRY ?= elastic-gpu-telemetry
TAG      ?= latest

.PHONY: build test test-load swagger docker-build docker-push lint clean

build:
	@mkdir -p $(BINDIR)
	@for cmd in $(CMDS); do \
		echo "Building $$cmd..."; \
		go build -o $(BINDIR)/$$cmd ./cmd/$$cmd; \
	done

test:
	go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

test-load:
	go test -tags loadtest -timeout 120s -run TestLoad ./internal/mq/

swagger:
	$(shell go env GOPATH)/bin/swag init -g cmd/api-gateway/main.go -o cmd/api-gateway/docs/

docker-build:
	@for cmd in $(CMDS); do \
		echo "Building $$cmd..."; \
		docker build -f deploy/docker/$$cmd.Dockerfile -t $(REGISTRY)/$$cmd:$(TAG) .; \
	done

docker-push: docker-build
	@for cmd in $(CMDS); do \
		echo "Pushing $(REGISTRY)/$$cmd:$(TAG)..."; \
		docker push $(REGISTRY)/$$cmd:$(TAG); \
	done

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINDIR) coverage.out coverage.html
