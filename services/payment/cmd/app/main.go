package main

import (
	"database/sql"
	"log"
	"os"

	"payment-service/internal/repository"
	httptransport "payment-service/internal/transport/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := getEnv("PAYMENT_DB_DSN", "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable")
	port := getEnv("PORT", "8081")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	paymentRepo := repository.NewPaymentPostgresRepository(db)
	paymentUC := usecase.NewPaymentUsecase(paymentRepo)
	handler := httptransport.NewHandler(paymentUC)

	r := gin.Default()
	handler.RegisterRoutes(r)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
