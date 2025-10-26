package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"PostsAndCommentsMicroservice/graph/model"

	"github.com/jackc/pgx/v5"
)

type PgBus struct {
	conn     *pgx.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
	handlers map[string][]func(model.Comment)
}

func NewPgBus(parentCtx context.Context, dsn string) (*PgBus, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		cancel()
		return nil, err
	}

	b := &PgBus{
		conn:     conn,
		ctx:      ctx,
		cancel:   cancel,
		handlers: make(map[string][]func(model.Comment)),
	}

	// Запускаем горутину, которая читает уведомления постоянно
	go b.listenLoop()
	return b, nil
}

func (b *PgBus) listenLoop() {
	// WaitForNotification блокирует и ждёт нотификаций
	for {
		// WaitForNotification возвращает (*pgconn.Notification, error)
		notif, err := b.conn.WaitForNotification(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				// скорее всего Context cancelled — graceful shutdown
				return
			}
			log.Printf("[PgBus] WaitForNotification error: %v", err)
			// Попробуем короткий backoff и продолжим
			// Можно реализовать reconnect при фатальной ошибке
			return
		}
		if notif == nil {
			continue
		}

		// логируем пришедшее уведомление
		log.Printf("[PgBus] recv notify channel=%s payload=%s", notif.Channel, notif.Payload)

		var c model.Comment
		if err := json.Unmarshal([]byte(notif.Payload), &c); err != nil {
			log.Printf("[PgBus] json unmarshal failed: %v", err)
			continue
		}

		// безопасно получить текущий слайс обработчиков
		b.mu.RLock()
		hs := append([]func(model.Comment){}, b.handlers[notif.Channel]...)
		b.mu.RUnlock()

		for _, h := range hs {
			go h(c) // асинхронно раздаём
		}
	}
}

func (b *PgBus) Publish(topic string, msg model.Comment) {
	payload, _ := json.Marshal(msg)
	chanName := pgx.Identifier{topic}.Sanitize()
	// Отправляем как параметр, чтобы избежать проблем с кавычками
	if _, err := b.conn.Exec(b.ctx, fmt.Sprintf("NOTIFY %s, $1", chanName), string(payload)); err != nil {
		log.Printf("[PgBus] Publish error topic=%s err=%v", topic, err)
	} else {
		log.Printf("[PgBus] Published topic=%s", topic)
	}
}

func (b *PgBus) Subscribe(topic string, h func(model.Comment)) Unsubscribe {
	// Выполним LISTEN на выделенном соединении (если ещё не слушаем)
	b.mu.Lock()
	if _, ok := b.handlers[topic]; !ok {
		if _, err := b.conn.Exec(b.ctx, "LISTEN "+pgx.Identifier{topic}.Sanitize()); err != nil {
			log.Printf("[PgBus] LISTEN error topic=%s err=%v", topic, err)
			// всё равно регистрируем локально — но уведомлений не будет
		}
	}
	b.handlers[topic] = append(b.handlers[topic], h)
	idx := len(b.handlers[topic]) - 1
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		arr := b.handlers[topic]
		if idx >= 0 && idx < len(arr) {
			arr[idx] = arr[len(arr)-1]
			b.handlers[topic] = arr[:len(arr)-1]
		}
		if len(b.handlers[topic]) == 0 {
			// если локальных слушателей не осталось — можно UNLISTEN
			if _, err := b.conn.Exec(b.ctx, "UNLISTEN "+pgx.Identifier{topic}.Sanitize()); err != nil {
				log.Printf("[PgBus] UNLISTEN error: %v", err)
			}
			delete(b.handlers, topic)
		}
		b.mu.Unlock()
	}
}

func (b *PgBus) Close() error {
	b.cancel()
	return b.conn.Close(b.ctx)
}
