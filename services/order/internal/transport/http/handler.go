package http

import (
	"errors"
	"net/http"
	"strconv"

	"order-service/internal/domain"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.OrderUsecase
}

func NewHandler(uc *usecase.OrderUsecase) *Handler {
	return &Handler{uc: uc}
}

type createOrderRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	ItemName   string `json:"item_name" binding:"required"`
	Amount     int64  `json:"amount" binding:"required"`
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/orders", h.createOrder)
	r.PATCH("/orders/:id/cancel", h.cancelOrder)
	r.GET("/orders", h.getFilteredList)
}

func (h *Handler) createOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	order, err := h.uc.CreateOrder(c.Request.Context(), req.CustomerID, req.ItemName, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidAmount):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrPaymentServiceUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "order": order})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"order": order})
}

func (h *Handler) cancelOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	order, err := h.uc.CancelOrder(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrInvalidOrderState):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel order"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"order": order})
}

func (h *Handler) getFilteredList(c *gin.Context) {
	minAmountStr := c.Query("min_amount")
	maxAmountStr := c.Query("max_amount")

	if minAmountStr == "" || maxAmountStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "min_amount and max_amount are required"})
		return
	}

	minAmount, err := strconv.ParseInt(minAmountStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "min_amount must be a valid integer"})
		return
	}

	maxAmount, err := strconv.ParseInt(maxAmountStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_amount must be a valid integer"})
		return
	}

	if minAmount < 5 || maxAmount > 100000 || minAmount > maxAmount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount values"})
		return
	}

	orders, err := h.uc.GetFilteredList(c.Request.Context(), minAmount, maxAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders})
}
