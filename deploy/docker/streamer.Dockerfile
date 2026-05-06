FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/streamer ./cmd/streamer

FROM alpine:3.19
COPY --from=builder /app/streamer /app/streamer
COPY data/ /data/
ENTRYPOINT ["/app/streamer"]
