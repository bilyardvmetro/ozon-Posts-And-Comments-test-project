package graph

import (
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/pubsub"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/store"
)

type Resolver struct {
	Store store.Store
	Bus   pubsub.Bus
}
