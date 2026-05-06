FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/mq ./cmd/mq

FROM alpine:3.19
COPY --from=builder /app/mq /app/mq
EXPOSE 7000
ENTRYPOINT ["/app/mq"]
