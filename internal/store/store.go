package store

import (
	"PostsAndCommentsMicroservice/graph/model"
	"context"
	"errors"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	// Posts
	CreatePost(ctx context.Context, post *model.Post) error
	GetPost(ctx context.Context, id string) (*model.Post, error)
	ListPosts(ctx context.Context) ([]*model.Post, error)
	CloseComments(ctx context.Context, id string, closed bool) (*model.Post, error)

	// Comments
	CreateComment(ctx context.Context, comment *model.Comment) error
	ListComments(ctx context.Context, postID string, parentID *string, after *string, limit int) (*model.CommentPage, error)
	BatchCommentsCount(ctx context.Context, postIDs []string) (map[string]int, error)
}
