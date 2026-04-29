package grpc

import (
	"order-service/internal/usecase"

	orderpb "github.com/Casper-242464/ConvertedProtosRepo/proto/order"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderTrackingServer struct {
	orderpb.UnimplementedOrderTrackingServer
	hub *usecase.OrderStatusHub
}

func NewOrderTrackingServer(hub *usecase.OrderStatusHub) *OrderTrackingServer {
	return &OrderTrackingServer{hub: hub}
}

func (s *OrderTrackingServer) SubscribeToOrderUpdates(req *orderpb.OrderRequest, stream orderpb.OrderTracking_SubscribeToOrderUpdatesServer) error {
	if req.GetOrderId() == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}

	statusCh, cancel := s.hub.Subscribe(req.GetOrderId())
	defer cancel()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case status, ok := <-statusCh:
			if !ok {
				return nil
			}
			if err := stream.Send(&orderpb.OrderStatusUpdate{Status: status, UpdatedAt: timestamppb.Now()}); err != nil {
				return err
			}
		}
	}
}
