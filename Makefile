BINDIR := bin
CMDS    := mq streamer collector api-gateway

.PHONY: build test test-load swagger docker-build lint clean

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
	docker build -f deploy/docker/mq.Dockerfile -t elastic-gpu-telemetry/mq:latest .
	docker build -f deploy/docker/streamer.Dockerfile -t elastic-gpu-telemetry/streamer:latest .
	docker build -f deploy/docker/collector.Dockerfile -t elastic-gpu-telemetry/collector:latest .
	docker build -f deploy/docker/api-gateway.Dockerfile -t elastic-gpu-telemetry/api-gateway:latest .

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINDIR) coverage.out coverage.html
