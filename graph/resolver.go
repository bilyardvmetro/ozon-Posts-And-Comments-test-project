package graph

import (
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/pubsub"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/store"
	"github.com/rs/zerolog"
)

type Resolver struct {
	Store  store.Store
	Bus    pubsub.Bus
	Logger zerolog.Logger
}
