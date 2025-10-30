package graph

import (
	"context"

	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/graph/model"
)

const maxCommentLen = 2000

func (r *Resolver) CommentsCount(ctx context.Context, post *model.Post) (int, error) {
	if post.CommentsCount != 0 {
		return post.CommentsCount, nil
	}

	loaders := GetLoaders(ctx)
	if loaders == nil || loaders.CommentsCount == nil {
		m, err := r.Store.BatchCommentsCount(ctx, []string{post.ID})
		if err != nil {
			return 0, err
		}
		return m[post.ID], nil
	}

	return loaders.CommentsCount.Load(ctx, post.ID)
}
