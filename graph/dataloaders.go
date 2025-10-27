package graph

import (
	"context"
	"sync"
	"time"

	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/store"
)

type ctxKey int

const loadersKey ctxKey = 1

type Loaders struct {
	CommentsCount *CommentsCountLoader
}

func WithLoaders(st store.Store, next func(ctx context.Context)) func(ctx context.Context) {
	return func(ctx context.Context) {
		loaders := &Loaders{
			CommentsCount: NewCommentsCountLoader(
				st,
				2*time.Millisecond,
				512,
			),
		}
		ctx = context.WithValue(ctx, loadersKey, loaders)
		next(ctx)
	}
}

func GetLoaders(ctx context.Context) *Loaders {
	if v := ctx.Value(loadersKey); v != nil {
		if loader, ok := v.(*Loaders); ok {
			return loader
		}
	}
	return nil
}

type CommentsCountLoader struct {
	st       store.Store
	mu       sync.Mutex
	cache    map[string]int
	pending  map[string][]chan result
	maxBatch int
	delay    time.Duration
}

type result struct {
	val int
	err error
}

func NewCommentsCountLoader(st store.Store, delay time.Duration, maxBatch int) *CommentsCountLoader {
	return &CommentsCountLoader{
		st:       st,
		cache:    make(map[string]int),
		pending:  make(map[string][]chan result),
		delay:    delay,
		maxBatch: maxBatch,
	}
}

func (l *CommentsCountLoader) Load(ctx context.Context, postID string) (int, error) {
	l.mu.Lock()
	if v, ok := l.cache[postID]; ok {
		l.mu.Unlock()
		return v, nil
	}

	ch := make(chan result, 1)
	l.pending[postID] = append(l.pending[postID], ch)

	if len(l.pending) == 1 {
		delay := l.delay
		if delay <= 0 {
			delay = time.Millisecond
		}
		go l.flushAfter(delay)
	}
	l.mu.Unlock()

	select {
	case res := <-ch:
		return res.val, res.err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (l *CommentsCountLoader) flushAfter(delay time.Duration) {
	time.Sleep(delay)
	l.mu.Lock()

	keys := make([]string, 0, len(l.pending))
	waiters := make(map[string][]chan result, len(l.pending))

	for k, arr := range l.pending {
		keys = append(keys, k)
		waiters[k] = arr
		if len(keys) >= l.maxBatch && l.maxBatch > 0 {
			break
		}
	}

	for _, k := range keys {
		delete(l.pending, k)
	}
	l.mu.Unlock()

	m, err := l.st.BatchCommentsCount(context.Background(), keys)

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, k := range keys {
		val := 0
		if err == nil {
			if v, ok := m[k]; ok {
				val = v
			}
			l.cache[k] = val
		}
		for _, ch := range waiters[k] {
			ch <- result{val: val, err: err}
			close(ch)
		}
	}
}
