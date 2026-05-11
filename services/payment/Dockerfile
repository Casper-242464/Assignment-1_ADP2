FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/payment-service ./cmd/app

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /out/payment-service /app/payment-service
EXPOSE 50051
ENTRYPOINT ["/app/payment-service"]
