package pubsub

import (
	"sync"
	"testing"
	"time"

	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph/model"

	"github.com/google/uuid"
)

// helper: быстро создаёт комментарий
func mkComment() model.Comment {
	return model.Comment{
		ID:        uuid.NewString(),
		PostID:    uuid.NewString(),
		Author:    "tester",
		Body:      "hi",
		Depth:     0,
		CreatedAt: time.Now().UTC(),
	}
}

func TestMemoryBus_PublishSingle(t *testing.T) {
	t.Parallel()
	b := NewMemoryBus()
	postID := uuid.NewString()

	got := make(chan model.Comment, 1)
	unsub := b.Subscribe(postID, func(c model.Comment) { got <- c })
	defer unsub()

	msg := mkComment()
	msg.PostID = postID

	b.Publish(postID, msg)

	select {
	case c := <-got:
		if c.ID != msg.ID {
			t.Fatalf("want %s got %s", msg.ID, c.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestMemoryBus_FanOutAndUnsubscribe(t *testing.T) {
	t.Parallel()
	b := NewMemoryBus()
	postID := uuid.NewString()

	const subs = 5
	var wg sync.WaitGroup
	wg.Add(subs)
	unsubs := make([]func(), 0, subs)
	for i := 0; i < subs; i++ {
		unsubs = append(unsubs, b.Subscribe(postID, func(c model.Comment) {
			wg.Done()
		}))
	}
	// опубликуем
	b.Publish(postID, mkComment())
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for fanout")
	}
	// отписываемся и проверяем, что больше не придут сообщения
	for _, u := range unsubs {
		u()
	}
	ch := make(chan struct{})
	go func() {
		b.Publish(postID, mkComment())
		close(ch)
	}()
	select {
	case <-ch:
		// даём время на доставку — если бы были подписчики, сообщение бы дошло
		time.Sleep(100 * time.Millisecond)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("publish blocked")
	}
}

func TestMemoryBus_ConcurrentPublish(t *testing.T) {
	t.Parallel()
	b := NewMemoryBus()
	postID := uuid.NewString()

	const subs = 8
	const pubs = 200
	var mu sync.Mutex
	var got int

	unsub := make([]func(), 0, subs)
	for i := 0; i < subs; i++ {
		u := b.Subscribe(postID, func(c model.Comment) {
			mu.Lock()
			got++
			mu.Unlock()
		})
		unsub = append(unsub, u)
	}
	defer func() {
		for _, u := range unsub {
			u()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(pubs)
	for i := 0; i < pubs; i++ {
		go func() {
			defer wg.Done()
			b.Publish(postID, mkComment())
		}()
	}
	wg.Wait()
	// позволим обработчикам отработать
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	total := got
	mu.Unlock()

	if total != pubs*subs {
		t.Fatalf("expected %d messages, got %d", pubs*subs, total)
	}
}
