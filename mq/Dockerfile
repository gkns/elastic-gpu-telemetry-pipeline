FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /mq ./cmd/main.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /mq /mq
EXPOSE 8080
ENTRYPOINT ["/mq"]
