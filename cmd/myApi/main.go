package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph/generated"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/logger"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/pubsub"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/store"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func main() {
	logger.Init()
	logger.Log.Info().Msg("Logger initialized")

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	storeType := os.Getenv("STORE")
	var st store.Store
	var err error

	switch storeType {
	case "pg":
		dsn := os.Getenv("POSTGRES_DSN")
		if dsn == "" {
			logger.Log.Fatal().Msg("POSTGRES_DSN environment variable not set")
		}

		st, err = store.NewPostgres(dsn)
		if err != nil {
			logger.Log.Fatal().Err(err).Msg("Failed to connect to postgres")
		}
	default:
		st = store.NewMemStore()
	}

	bus := pubsub.NewMemoryBus()
	resolvers := &graph.Resolver{Store: st, Bus: bus, Logger: logger.Log}
	server := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: resolvers}))

	server.AddTransport(transport.POST{})
	server.AddTransport(transport.GET{})
	server.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 30 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin:  func(r *http.Request) bool { return true },
			Subprotocols: []string{"graphql-transport-ws", "graphql-ws"},
		},
	})
	server.Use(extension.Introspection{})

	logger.AttachGraphQLHooks(server)
	server.SetErrorPresenter(func(ctx context.Context, e error) *gqlerror.Error {
		code := "INTERNAL"
		msg := e.Error()

		switch {
		case strings.Contains(msg, "forbidden"):
			code = "FORBIDDEN"
		case strings.Contains(msg, "not found"):
			code = "NOT_FOUND"
		case strings.Contains(msg, "too long"), strings.Contains(msg, "required"), strings.Contains(msg, "invalid"):
			code = "BAD_REQUEST"
		}

		ge := graphql.DefaultErrorPresenter(ctx, e)
		ge.Message = msg
		ge.Extensions = map[string]any{"code": code}
		return ge
	})

	cors := corsMiddleware(os.Getenv("CORS_ORIGINS"))
	mux := http.NewServeMux()
	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	mux.Handle("/query", cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graph.WithLoaders(st, func(ctx context.Context) {
			server.ServeHTTP(w, r.WithContext(ctx))
		})(r.Context())
	})))

	addr := ":8080"
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Log.Info().Str("addr", addr).Msg("starting server")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal().Err(err).Msg("http server stopped")
		}
	}()

	<-rootCtx.Done()
	logger.Log.Info().Msg("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("http server shutdown error")
	} else {
		logger.Log.Info().Msg("http server shutdown successfully")
	}

	closeIfNeeded(bus, "subscription bus")
	closeIfNeeded(st, "store")

	logger.Log.Info().Msg("graceful shutdown complete")
}

func closeIfNeeded(x any, name string) {
	if c, ok := x.(io.Closer); ok && c != nil {
		if err := c.Close(); err != nil {
			logger.Log.Error().Str("component", name).Msg("close error")
			return
		}
		logger.Log.Info().Str("component", name).Msg("closed")
	}
}

func corsMiddleware(origins string) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if len(allowed) == 0 {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
