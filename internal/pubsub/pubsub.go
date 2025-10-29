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
	mu     sync.RWMutex
	m      map[string][]handler
	seq    int64
	closed bool
}

func NewMemoryBus() Bus {
	return &memoryBus{m: make(map[string][]handler)}
}

func (m *memoryBus) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	m.m = nil
	return nil
}

func (m *memoryBus) Publish(postID string, msg model.Comment) {
	m.mu.RLock()
	if m.closed || m.m == nil {
		m.mu.RUnlock()
		return
	}

	hs := append([]handler(nil), m.m[postID]...)
	m.mu.RUnlock()

	for _, handler := range hs {
		go handler.fn(msg)
	}
}

func (m *memoryBus) Subscribe(postID string, h func(model.Comment)) Unsubscribe {
	m.mu.Lock()
	if m.closed || m.m == nil {
		m.mu.RUnlock()
		return func() {}
	}

	m.seq++
	id := m.seq
	m.m[postID] = append(m.m[postID], handler{id: id, fn: h})
	m.mu.Unlock()

	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.closed || m.m == nil {
			m.mu.RUnlock()
			return
		}

		handlers := m.m[postID]
		for i := range handlers {
			if handlers[i].id == id {
				handlers[i] = handlers[len(handlers)-1]
				m.m[postID] = handlers[:len(handlers)-1]

				if len(m.m[postID]) == 0 {
					delete(m.m, postID)
				}
				break
			}
		}
	}
}
