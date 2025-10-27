package pubsub

import (
	"sync"

	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph/model"
)

type Unsubscribe func()

type Bus interface {
	Publish(topic string, msg model.Comment)
	Subscribe(topic string, h func(model.Comment)) Unsubscribe
}

type handler struct {
	id int64
	fn func(model.Comment)
}

type memoryBus struct {
	mu  sync.RWMutex
	m   map[string][]handler
	seq int64
}

func NewMemoryBus() Bus {
	return &memoryBus{m: make(map[string][]handler)}
}

func (m *memoryBus) Publish(postID string, msg model.Comment) {
	m.mu.RLock()
	hs := append([]handler(nil), m.m[postID]...)
	m.mu.RUnlock()

	for _, handler := range hs {
		go handler.fn(msg)
	}
}

func (m *memoryBus) Subscribe(postID string, h func(model.Comment)) Unsubscribe {
	m.mu.Lock()
	m.seq++
	id := m.seq
	m.m[postID] = append(m.m[postID], handler{id: id, fn: h})
	m.mu.Unlock()

	return func() {
		m.mu.Lock()
		handlers := m.m[postID]
		for i := range handlers {
			if handlers[i].id == id {
				handlers[i] = handlers[len(handlers)-1]
				m.m[postID] = handlers[:len(handlers)-1]
				break
			}
		}
		m.mu.Unlock()
	}
}
