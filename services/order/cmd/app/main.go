package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"order-service/internal/repository"
	httptransport "order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable"
	paymentURL := "http://localhost:8081"
	port := "8080"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	orderRepo := repository.NewOrderPostgresRepository(db)
	paymentClient := httptransport.NewPaymentClient(paymentURL, &http.Client{Timeout: 2 * time.Second})
	orderUC := usecase.NewOrderUsecase(orderRepo, paymentClient)
	handler := httptransport.NewHandler(orderUC)

	r := gin.Default()
	handler.RegisterRoutes(r)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
