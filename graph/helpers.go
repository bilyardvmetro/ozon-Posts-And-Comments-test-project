package graph

import (
	"context"

	"github.com/bilyardvmetro/ozon-Posts-And-Comments-test-project/internal/auth"
)

const maxCommentLen = 2000

func currentUserName(ctx context.Context) string {
	if u := auth.FromContext(ctx); u != nil {
		return u.Name
	}
	return "" // гость
}
