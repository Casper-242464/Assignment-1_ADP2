# ADP2 Assignment 1 - Order and Payment Microservices

This repository contains two Go microservices that model a simple checkout flow:
- `order-service` manages order creation and cancellation.
- `payment-service` authorizes or declines payments.

The design follows DDD-inspired bounded contexts with a clean architecture layering style inside each service.

## 1) Architecture Decisions

### Service decomposition
- Split by business capability, not technical layer:
  - `services/order` owns order lifecycle.
  - `services/payment` owns payment authorization records.
- Each service has its own database schema and migration (`migrations/001_init.sql`), which keeps data ownership explicit.

### Internal layering (per service)
Each service uses the same package structure:
- `internal/domain`: Entities, domain errors, and interfaces (ports).
- `internal/usecase`: Application/business logic.
- `internal/repository`: Postgres adapters for persistence.
- `internal/transport/http`: Gin HTTP handlers (and HTTP client adapter in order-service).
- `cmd/app`: Composition root (wiring dependencies, env config, startup).

This creates clear dependency flow:
- Transport -> Usecase -> Domain interfaces
- Repository/HTTP clients implement domain interfaces

### Inter-service communication
- `order-service` synchronously calls `payment-service` over HTTP (`POST /payments`) during order creation.
- This is a deliberate, simple consistency model for assignment scope.
- A 2-second HTTP timeout is configured in order-service to avoid hanging requests.

### State model decisions
- Order states: `Pending`, `Paid`, `Failed`, `Cancelled`.
- Payment states: `Authorized`, `Declined`.
- If payment is declined, order is still created but becomes `Failed`.
- Cancellation is allowed only while order is `Pending`.

## 2) Bounded Contexts

### Order Context (`services/order`)
Responsibilities:
- Accept order requests.
- Generate order IDs.
- Persist order records.
- Orchestrate payment charge.
- Enforce order state transition rules (e.g., cancellation only when pending).

Core concepts:
- Aggregate/entity: `Order`
- Inbound operations:
  - `POST /orders`
  - `PATCH /orders/:id/cancel`
- External dependency (port): `PaymentGateway`

Order context does not know payment storage internals. It only depends on the payment contract response (`status`, `transaction_id`).

### Payment Context (`services/payment`)
Responsibilities:
- Accept payment requests tied to an order ID.
- Apply authorization rule:
  - Amount > 100000 cents => `Declined`
  - Otherwise => `Authorized`
- Persist payment records.
- Expose lookup by `order_id`.

Core concepts:
- Aggregate/entity: `Payment`
- Inbound operations:
  - `POST /payments`
  - `GET /payments/:order_id`

Payment context is intentionally independent from order storage and only needs `order_id` as a reference.

## 3) Failure Handling

### Input validation failures
- Invalid JSON/missing required fields -> `400 Bad Request`
- Invalid amount (<= 0) -> `400 Bad Request`

### Downstream payment service failures (order-service)
When creating an order:
- If payment call times out/network fails -> order status is updated to `Failed`, API returns `503 Service Unavailable`.
- If payment service returns `5xx` -> treated as unavailable (`503`).
- If payment service returns non-201 non-5xx (e.g., `400`) -> treated as order creation failure (`500`) by current handler mapping.

### Repository/database failures
- Unhandled persistence failures bubble up as `500 Internal Server Error`.
- Not found cases are mapped explicitly:
  - Order not found -> `404`
  - Payment not found -> `404`

### Compensating behavior
- There is no distributed transaction (no 2PC/Saga orchestrator).
- Compensation is local and explicit: if payment fails during order creation, order is marked `Failed`.
- This keeps behavior deterministic while avoiding cross-service DB coupling.

## 4) API Summary

### Order Service (default `http://localhost:8080`)
- `POST /orders`
- `PATCH /orders/:id/cancel`

### Payment Service (default `http://localhost:8081`)
- `POST /payments`
- `GET /payments/:order_id`

## 5) Postman Examples

An importable Postman collection is provided at:
- `postman/ADP2-Microservices.postman_collection.json`

### Collection variables
- `order_base_url` = `http://localhost:8080`
- `payment_base_url` = `http://localhost:8081`
- `order_id` (set automatically by test script on create order)

### Example flows included
1. Create Order (expected `Paid`)
2. Create Order (expected `Failed` due to payment decline)
3. Cancel Order by ID
4. Create Payment directly
5. Get Payment by order ID

## 6) Run Notes

Default environment variables:
- Order service:
  - `PORT` (default `8080`)
  - `ORDER_DB_DSN`
  - `PAYMENT_SERVICE_URL` (default `http://localhost:8081`)
- Payment service:
  - `PORT` (default `8081`)
  - `PAYMENT_DB_DSN`

Start payment service before order service so order creation can call payments successfully.
