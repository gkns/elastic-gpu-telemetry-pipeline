FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/api-gateway ./cmd/api-gateway

FROM alpine:3.19
COPY --from=builder /app/api-gateway /app/api-gateway
EXPOSE 8080
ENTRYPOINT ["/app/api-gateway"]
