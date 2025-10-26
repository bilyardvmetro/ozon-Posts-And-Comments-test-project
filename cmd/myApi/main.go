package main

import (
	"PostsAndCommentsMicroservice/graph"
	"PostsAndCommentsMicroservice/graph/generated"
	"PostsAndCommentsMicroservice/internal/auth"
	"PostsAndCommentsMicroservice/internal/pubsub"
	"PostsAndCommentsMicroservice/internal/store"
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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

	cors := corsMiddleware(os.Getenv("CORS_ORIGINS"))

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

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", cors(auth.WithUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graph.WithLoaders(st, func(ctx context.Context) {
			server.ServeHTTP(w, r.WithContext(ctx))
		})(r.Context())
	}))))

	log.Printf("listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
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
