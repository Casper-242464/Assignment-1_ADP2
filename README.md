# ADP2 Assignment 2 - Switch from REST to gRPC

This project contains two Go microservices:

- `order-service`: exposes REST endpoints for order operations and uses gRPC to call the payment service.
- `payment-service`: exposes a gRPC server for payment processing.

The order service also exposes a gRPC tracking server for order status updates.

## Proto Repositories

The shared protobuf-generated Go packages used by both services come from:

- Shared proto repository: https://github.com/Casper-242464/ConvertedProtosRepo
- Payment proto package: https://github.com/Casper-242464/ConvertedProtosRepo/tree/main/proto/payment
- Order proto package: https://github.com/Casper-242464/ConvertedProtosRepo/tree/main/proto/order

## Services and Ports

- `order-service` REST API: `http://localhost:8080`
- `order-service` gRPC tracking server: `localhost:50052`
- `payment-service` gRPC server: `localhost:50051`
- PostgreSQL database for orders: `order_db`
- PostgreSQL database for payments: `payment_db`

## How to Run

### 1. Prerequisites

Make sure these are installed locally:

- Go `1.24`
- PostgreSQL

### 2. Create the databases

Create two PostgreSQL databases:

- `order_db`
- `payment_db`

### 3. Run the SQL migrations

Apply the migration files:

- `services/order/migrations/001_init.sql` on `order_db`
- `services/payment/migrations/001_init.sql` on `payment_db`

Example using `psql`:

```powershell
psql -U postgres -d order_db -f services/order/migrations/001_init.sql
psql -U postgres -d payment_db -f services/payment/migrations/001_init.sql
```

### 4. Start the payment service first

The order service depends on the payment gRPC server at `localhost:50051`.

```powershell
cd services/payment
go mod download
go run ./cmd/app
```

### 5. Start the order service

Open a second terminal:

```powershell
cd services/order
go mod download
go run ./cmd/app
```

### 6. Test the REST API

The order service exposes these HTTP endpoints:

- `POST /orders`
- `PATCH /orders/:id/cancel`
- `GET /orders?min_amount=<value>&max_amount=<value>`

Example request:

```powershell
curl -X POST http://localhost:8080/orders `
  -H "Content-Type: application/json" `
  -d "{\"customer_id\":\"cust-001\",\"item_name\":\"Mechanical Keyboard\",\"amount\":25000}"
```

The Postman collection is available here:

- `postman/postman_examples.json`

## Architecture Diagram

```mermaid
flowchart LR
    C[Client / Postman]
    OHTTP[Order Service REST API<br/>:8080]
    OGRPC[Order Tracking gRPC Server<br/>:50052]
    PGRPC[Payment Service gRPC Server<br/>:50051]
    ODB[(order_db)]
    PDB[(payment_db)]
    G1[PaymentService.ProcessPayment]
    G2[OrderTracking.SubscribeToOrderUpdates]

    C -->|HTTP REST| OHTTP
    OHTTP -->|SQL| ODB
    OHTTP -->|gRPC unary| G1
    G1 --> PGRPC
    PGRPC -->|SQL| PDB
    C -.->|gRPC stream| G2
    G2 --> OGRPC
```

## Communication Summary

- Client to order service: HTTP/REST
- Order service to payment service: gRPC unary call
- Client to order tracking server: gRPC server-streaming call
- Services to databases: PostgreSQL

## Notes

- The current `payment-service` startup code runs a gRPC server, not an HTTP server.
- Because of that, order creation is the main entry point for payment processing in the current setup.
