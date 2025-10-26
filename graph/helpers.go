package graph

import (
	"PostsAndCommentsMicroservice/internal/auth"
	"context"
)

const maxCommentLen = 2000

func currentUserName(ctx context.Context) string {
	if u := auth.FromContext(ctx); u != nil {
		return u.Name
	}
	return "" // гость
}
