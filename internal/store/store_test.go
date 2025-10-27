package store_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph/model"
	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/store"
)

func TestMemoryStore_CreateAndCountComments(t *testing.T) {
	m := store.NewMemStore().(*store.MemStore)
	ctx := context.Background()

	// создаём пост
	pid := "post-1"
	m.Posts[pid] = &model.Post{ID: pid, Title: "t", Body: "b", Author: "a", CreatedAt: time.Now().UTC()}

	// добавляем комментарии
	c1 := &model.Comment{ID: "c1", PostID: pid, Body: "one", Author: "u1", CreatedAt: time.Now().UTC()}
	c2 := &model.Comment{ID: "c2", PostID: pid, Body: "two", Author: "u2", CreatedAt: time.Now().UTC()}
	if err := m.CreateComment(ctx, c1); err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if err := m.CreateComment(ctx, c2); err != nil {
		t.Fatalf("create c2: %v", err)
	}

	// GetPost должен посчитать commentsCount
	p, err := m.GetPost(ctx, pid)
	if err != nil {
		t.Fatalf("getpost: %v", err)
	}
	if p.CommentsCount != 2 {
		t.Fatalf("expected commentsCount 2 got %d", p.CommentsCount)
	}
}

func TestMemoryStore_ListComments_PaginationCursor(t *testing.T) {
	m := store.NewMemStore().(*store.MemStore)
	ctx := context.Background()

	pid := "post-2"
	m.Posts[pid] = &model.Post{ID: pid, Title: "t2", Body: "b", Author: "a", CreatedAt: time.Now().UTC()}

	// три комментария по времени
	base := time.Now().UTC().Add(-time.Second)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("id%d", i)
		m.Comments[id] = &model.Comment{
			ID:        id,
			PostID:    pid,
			Author:    "u",
			Body:      id,
			Depth:     0,
			CreatedAt: base.Add(time.Duration(i) * time.Millisecond),
		}
	}

	first := 2
	conn, err := m.ListComments(ctx, pid, nil, nil, first)
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(conn.Edges) != 2 {
		t.Fatalf("expected 2 edges got %d", len(conn.Edges))
	}
	if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == nil {
		t.Fatal("expected hasNext with endCursor")
	}

	// вторая страница
	conn2, err := m.ListComments(ctx, pid, nil, conn.PageInfo.EndCursor, 10)
	if err != nil {
		t.Fatalf("list2: %v", err)
	}
	if len(conn2.Edges) != 1 {
		t.Fatalf("expected 1 edge got %d", len(conn2.Edges))
	}
	// Проверим, что курсор корректно кодируется/декодируется
	raw, _ := base64.StdEncoding.DecodeString(*conn.PageInfo.EndCursor)
	if len(raw) == 0 {
		t.Fatal("endCursor decode empty")
	}
}
