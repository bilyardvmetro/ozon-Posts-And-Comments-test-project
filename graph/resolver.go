package graph

import (
	"PostsAndCommentsMicroservice/internal/pubsub"
	"PostsAndCommentsMicroservice/internal/store"
)

type Resolver struct {
	Store store.Store
	Bus   pubsub.Bus
}
