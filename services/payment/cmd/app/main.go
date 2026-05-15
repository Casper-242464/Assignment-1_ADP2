package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"payment-service/internal/messaging"
	"payment-service/internal/repository"
	grpcTransport "payment-service/internal/transport/grpc"
	"payment-service/internal/usecase"

	paymentpb "github.com/Casper-242464/ConvertedProtosRepo/proto/payment"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	loadDotEnv()

	dsn := envOrDefault("PAYMENT_DB_DSN", "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable")
	grpcAddress := requireEnv("PAYMENT_GRPC_ADDRESS")
	grpcPort := requireEnv("PAYMENT_GRPC_PORT")
	rabbitURL := envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	paymentCompletedQueue := envOrDefault("PAYMENT_COMPLETED_QUEUE", "payment.completed")
	rabbitPoolSizeStr := envOrDefault("RABBITMQ_POOL_SIZE", "10")
	rabbitPoolSize, _ := strconv.Atoi(rabbitPoolSizeStr)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	publisher, err := messaging.NewRabbitMQPublisher(rabbitURL, paymentCompletedQueue, rabbitPoolSize)
	if err != nil {
		log.Fatalf("connect rabbitmq: %v", err)
	}
	defer publisher.Close()

	paymentRepo := repository.NewPaymentPostgresRepository(db)
	paymentUC := usecase.NewPaymentUsecase(paymentRepo, publisher)
	handler := grpcTransport.NewHandler(paymentUC)

	listener, err := net.Listen("tcp", net.JoinHostPort(grpcAddress, grpcPort))
	if err != nil {
		log.Fatalf("listen grpc: %v", err)
	}

	server := grpc.NewServer(grpc.UnaryInterceptor(loggingInterceptor))
	paymentpb.RegisterPaymentServiceServer(server, handler)

	log.Printf("payment gRPC server listening on %s", listener.Addr().String())
	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("serve grpc: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down payment service")
	server.GracefulStop()
}

func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Printf("gRPC %s took %s, error=%v", info.FullMethod, time.Since(start), err)
	return resp, err
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
