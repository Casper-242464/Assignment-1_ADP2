package usecase

import "sync"

type OrderStatusHub struct {
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

func NewOrderStatusHub() *OrderStatusHub {
	return &OrderStatusHub{subscribers: make(map[string][]chan string)}
}

func (h *OrderStatusHub) Subscribe(orderID string) (<-chan string, func()) {
	ch := make(chan string, 1)

	h.mu.Lock()
	h.subscribers[orderID] = append(h.subscribers[orderID], ch)
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		list := h.subscribers[orderID]
		for i, subscriber := range list {
			if subscriber == ch {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(list) == 0 {
			delete(h.subscribers, orderID)
		} else {
			h.subscribers[orderID] = list
		}
		close(ch)
	}

	return ch, cancel
}

func (h *OrderStatusHub) Publish(orderID string, status string) {
	h.mu.RLock()
	subs := append([]chan string(nil), h.subscribers[orderID]...)
	h.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- status:
		default:
		}
	}
}
