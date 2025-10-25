package pubsub

import "PostsAndCommentsMicroservice/graph/model"

type unsubscribe func()

type Bus interface {
	Publish(topic string, msg model.Comment)
	Subscribe(topic string, h func(model.Comment)) unsubscribe
}

type memoryBus struct {
	m map[string][]func(model.Comment)
}

func NewMemoryBus() Bus {
	return &memoryBus{m: map[string][]func(model.Comment){}}
}

func (m *memoryBus) Publish(postID string, msg model.Comment) {
	if handlers, ok := m.m[postID]; ok {
		for _, handler := range handlers {
			go handler(msg)
		}
	}
}

func (m *memoryBus) Subscribe(postID string, handler func(model.Comment)) unsubscribe {
	m.m[postID] = append(m.m[postID], handler)
	index := len(m.m[postID]) - 1

	return func() {
		handlers := m.m[postID]
		if index >= 0 && index < len(handlers) {
			m.m[postID] = append(handlers[:index], handlers[index+1:]...)
		}
	}
}
