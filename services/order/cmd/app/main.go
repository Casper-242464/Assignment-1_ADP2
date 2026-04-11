package main

import (
	"database/sql"
	"log"
	"net"

	"order-service/internal/repository"
	grpcTransport "order-service/internal/transport/grpc"
	httptransport "order-service/internal/transport/http"
	"order-service/internal/usecase"

	orderpb "github.com/Casper-242464/ConvertedProtosRepo/proto/order"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	dsn := "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable"
	paymentAddr := "localhost:50051"
	port := "8080"
	grpcPort := "50052"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	orderRepo := repository.NewOrderPostgresRepository(db)

	conn, err := grpcLib.Dial(paymentAddr, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial payment grpc: %v", err)
	}
	defer conn.Close()

	paymentClient := grpcTransport.NewPaymentClient(conn)
	statusHub := usecase.NewOrderStatusHub()
	orderUC := usecase.NewOrderUsecase(orderRepo, paymentClient, statusHub)
	handler := httptransport.NewHandler(orderUC)

	go func() {
		listener, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatalf("listen grpc: %v", err)
		}

		server := grpcLib.NewServer()
		orderpb.RegisterOrderTrackingServer(server, grpcTransport.NewOrderTrackingServer(statusHub))

		log.Printf("order gRPC tracking server listening on %s", grpcPort)
		if err := server.Serve(listener); err != nil {
			log.Fatalf("serve grpc: %v", err)
		}
	}()

	r := gin.Default()
	handler.RegisterRoutes(r)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
