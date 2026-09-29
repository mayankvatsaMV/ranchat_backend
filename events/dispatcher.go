package events

import (
	"context"
	"ranchat/utils"
	"sync"
)

// Return error so Publish/DoWithRetry knows whether to retry
type EventHandler func(payload any) error

type EventDispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		handlers: make(map[string][]EventHandler),
	}
}

func (d *EventDispatcher) Subscribe(eventName string, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventName] = append(d.handlers[eventName], handler)
}

func (d *EventDispatcher) Publish(eventName string, payload interface{}) {
	d.mu.RLock()
	handlers := append([]EventHandler(nil), d.handlers[eventName]...)
	d.mu.RUnlock()

	for _, handler := range handlers {
		h := handler
		go func() {
			ctx := context.Background()
			_ = utils.DoWithRetry(ctx, eventName, utils.DefaultRetryConfig, func() error {
				return h(payload)
			})
		}()
	}
}
