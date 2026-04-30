FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/order-service ./cmd/app

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /out/order-service /app/order-service
EXPOSE 8080 50052
ENTRYPOINT ["/app/order-service"]
