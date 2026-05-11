package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"order-service/internal/repository"
	grpcTransport "order-service/internal/transport/grpc"
	httptransport "order-service/internal/transport/http"
	"order-service/internal/usecase"

	orderpb "github.com/Casper-242464/ConvertedProtosRepo/proto/order"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	loadDotEnv()

	dsn := envOrDefault("ORDER_DB_DSN", "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable")
	httpPort := envOrDefault("ORDER_HTTP_PORT", "8080")
	orderGRPCAddress := requireEnv("ORDER_GRPC_ADDRESS")
	orderGRPCPort := requireEnv("ORDER_GRPC_PORT")
	paymentGRPCAddress := requireEnv("PAYMENT_GRPC_ADDRESS")
	paymentGRPCPort := requireEnv("PAYMENT_GRPC_PORT")
	redisAddress := envOrDefault("REDIS_ADDR", "localhost:6379")
	cacheTTL := envDurationOrDefault("ORDER_CACHE_TTL", 5*time.Minute)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	orderRepo := repository.NewOrderPostgresRepository(db)
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddress})
	defer redisClient.Close()
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("ping redis: %v", err)
	}
	orderCache := repository.NewOrderRedisCache(redisClient, cacheTTL)

	paymentTarget := net.JoinHostPort(paymentGRPCAddress, paymentGRPCPort)
	conn, err := grpcLib.Dial(paymentTarget, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial payment grpc: %v", err)
	}
	defer conn.Close()

	paymentClient := grpcTransport.NewPaymentClient(conn)
	statusHub := usecase.NewOrderStatusHub()
	orderUC := usecase.NewOrderUsecase(orderRepo, orderCache, paymentClient, statusHub)
	handler := httptransport.NewHandler(orderUC)

	listener, err := net.Listen("tcp", net.JoinHostPort(orderGRPCAddress, orderGRPCPort))
	if err != nil {
		log.Fatalf("listen grpc: %v", err)
	}

	server := grpcLib.NewServer()
	orderpb.RegisterOrderTrackingServer(server, grpcTransport.NewOrderTrackingServer(statusHub))

	go func() {
		log.Printf("order gRPC tracking server listening on %s", listener.Addr().String())
		if err := server.Serve(listener); err != nil {
			log.Fatalf("serve grpc: %v", err)
		}
	}()

	r := gin.Default()
	handler.RegisterRoutes(r)

	log.Printf("order REST server listening on :%s", httpPort)
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: r,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("run server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down order service")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown REST server: %v", err)
	}
	server.GracefulStop()
}

func loadDotEnv() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatalf("load .env: %v", err)
	}
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("%s must be a valid duration: %v", key, err)
	}
	return duration
}
