package main

import (
	"PostsAndCommentsMicroservice/graph"
	"PostsAndCommentsMicroservice/graph/generated"
	"PostsAndCommentsMicroservice/internal/pubsub"
	"PostsAndCommentsMicroservice/internal/store"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	storeType := os.Getenv("STORE")
	var st store.Store
	var err error

	switch storeType {
	case "pg":
		dsn := os.Getenv("POSTGRES_DSN")
		if dsn == "" {
			log.Fatalf("POSTGRES_DSN environment variable not set")
		}

		st, err = store.NewPostgres(dsn)
		if err != nil {
			log.Fatalf("Failed to connect to postgres: %v", err)
		}
	default:
		st = store.NewMemStore()
	}

	bus := pubsub.NewMemoryBus()
	resolvers := &graph.Resolver{Store: st, Bus: bus}

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

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graph.WithLoaders(st, func(ctx context.Context) {
			server.ServeHTTP(w, r.WithContext(ctx))
		})(r.Context())
	}))

	log.Printf("listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
