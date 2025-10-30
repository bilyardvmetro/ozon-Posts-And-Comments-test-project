package integration

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph/generated"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/logger"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/pubsub"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/store"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
)

func startTestServer(t *testing.T) (*http.Server, string, func()) {
	t.Helper()
	logger.Init()

	st := store.NewMemStore()
	bus := pubsub.NewMemoryBus()

	resolvers := &graph.Resolver{Store: st, Bus: bus, Logger: logger.Log}
	server := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: resolvers}))
	server.AddTransport(transport.POST{})
	server.Use(extension.Introspection{})

	mux := http.NewServeMux()
	mux.Handle("/query", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graph.WithLoaders(st, func(ctx context.Context) {
			server.ServeHTTP(w, r.WithContext(ctx))
		})(r.Context())
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	httpSrv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// запускаем
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = httpSrv.Serve(ln) // закрытие ожидаем в Shutdown
	}()

	addr := "http://" + ln.Addr().String()
	stop := func() {
		// закрываем красиво
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		wg.Wait()
	}
	return httpSrv, addr, stop
}

// --- ТЕСТЫ ---

// 1) Shutdown ждёт "длинный" запрос и закрывает порт для новых подключений
func TestGracefulShutdown_WaitsAndStopsAcceptingNew(t *testing.T) {
	httpSrv, addr, stop := startTestServer(t)
	defer stop()

	// сделаем "длинный" обработчик на время теста
	orig := httpSrv.Handler
	httpSrv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		orig.ServeHTTP(w, r)
	})

	// запустим долгий запрос
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, _ := http.NewRequest(http.MethodPost, addr+"/query", strings.NewReader(`{"query":"{__typename}"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("long request failed prematurely: %v", err)
			return
		}
		defer resp.Body.Close()
		_, _ = io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	}()

	// через 100ms попросим graceful shutdown (должен дождаться запроса ~300ms)
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := httpSrv.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	// долгий запрос должен завершиться успешно
	wg.Wait()

	// новые подключения после Shutdown должны отвергаться/не устанавливаться
	_, err = http.Get(addr + "/query")
	if err == nil {
		t.Fatalf("expected connection error after shutdown, got nil")
	}
}
