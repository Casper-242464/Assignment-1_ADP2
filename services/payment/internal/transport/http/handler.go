package http

import (
	"errors"
	"net/http"

	"payment-service/internal/domain"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.PaymentUsecase
}

func NewHandler(uc *usecase.PaymentUsecase) *Handler {
	return &Handler{uc: uc}
}

type createPaymentRequest struct {
	OrderID string `json:"order_id" binding:"required"`
	Amount  int64  `json:"amount" binding:"required"`
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/payments", h.createPayment)
	r.GET("/payments/:order_id", h.getPaymentByOrder)
}

func (h *Handler) createPayment(c *gin.Context) {
	var req createPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	payment, err := h.uc.CreatePayment(c.Request.Context(), req.OrderID, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidAmount):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"order_id":       payment.OrderID,
		"status":         payment.Status,
		"transaction_id": payment.TransactionID,
	})
}

func (h *Handler) getPaymentByOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_id"})
		return
	}

	payment, err := h.uc.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPaymentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id":       payment.OrderID,
		"status":         payment.Status,
		"transaction_id": payment.TransactionID,
		"amount":         payment.Amount,
	})
}
