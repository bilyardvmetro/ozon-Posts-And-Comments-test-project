package tests_test

import (
	"PostsAndCommentsMicroservice/graph"
	"PostsAndCommentsMicroservice/internal/pubsub"
	"PostsAndCommentsMicroservice/internal/store"
	"context"
	"strings"
	"testing"
)

func newResolverForTests() *graph.Resolver {
	return &graph.Resolver{
		Store: store.NewMemStore(),
		Bus:   pubsub.NewMemoryBus(),
	}
}

func TestCreatePostAndAddComment(t *testing.T) {
	r := newResolverForTests()
	ctx := context.Background()

	p, err := r.Mutation().CreatePost(ctx, "T", "B", "author")
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if p.ID == "" {
		t.Fatalf("post id empty")
	}

	// Добавим валидный комментарий
	c, err := r.Mutation().AddComment(ctx, p.ID, nil, "hello", "bob")
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}
	if c.PostID != p.ID {
		t.Fatalf("wrong post id")
	}

	// Проверим, что Comments возвращает его
	conn, err := r.Query().Comments(ctx, p.ID, nil, nil, nil)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 comment got %d", len(conn.Edges))
	}
}

func TestAddComment_Validation(t *testing.T) {
	r := newResolverForTests()
	ctx := context.Background()

	p, _ := r.Mutation().CreatePost(ctx, "t", "b", "u")
	// пустой
	if _, err := r.Mutation().AddComment(ctx, p.ID, nil, "   ", "bob"); err == nil {
		t.Fatal("expected empty body error")
	}
	// слишком длинный
	long := strings.Repeat("x", 2001)
	if _, err := r.Mutation().AddComment(ctx, p.ID, nil, long, "bob"); err == nil {
		t.Fatal("expected too long error")
	}
}

func TestAddComment_WithParentAndDepth(t *testing.T) {
	r := newResolverForTests()
	ctx := context.Background()

	p, _ := r.Mutation().CreatePost(ctx, "t", "b", "u")
	root, _ := r.Mutation().AddComment(ctx, p.ID, nil, "root", "a")
	child, err := r.Mutation().AddComment(ctx, p.ID, &root.ID, "child", "b")
	if err != nil {
		t.Fatalf("add child: %v", err)
	}
	if child.Depth != root.Depth+1 {
		t.Fatalf("expected depth %d got %d", root.Depth+1, child.Depth)
	}
}

func TestToggleCommentsClosed(t *testing.T) {
	r := newResolverForTests()
	ctx := context.Background()

	p, _ := r.Mutation().CreatePost(ctx, "t", "b", "u")
	if p.CommentsClosed {
		t.Fatalf("expected comments open by default")
	}
	// toggle to closed
	np, err := r.Mutation().ToggleCommentsClosed(ctx, p.ID, true)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !np.CommentsClosed {
		t.Fatalf("expected comments closed")
	}
}
