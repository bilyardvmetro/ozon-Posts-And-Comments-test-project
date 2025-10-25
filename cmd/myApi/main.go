package main

import (
	"PostsAndCommentsMicroservice/graph"
	"PostsAndCommentsMicroservice/graph/generated"
	"PostsAndCommentsMicroservice/internal/pubsub"
	"PostsAndCommentsMicroservice/internal/store"
	"log"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
)

func main() {
	st := store.NewMemStore()
	bus := pubsub.NewMemoryBus()

	resolvers := &graph.Resolver{Store: st, Bus: bus}
	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolvers}))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", server)

	log.Printf("listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
