FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/collector ./cmd/collector

FROM alpine:3.19
COPY --from=builder /app/collector /app/collector
ENTRYPOINT ["/app/collector"]
