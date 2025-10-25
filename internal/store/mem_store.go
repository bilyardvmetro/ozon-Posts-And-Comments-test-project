package store

import (
	"PostsAndCommentsMicroservice/graph/model"
	"context"
	"encoding/base64"
	"sort"
	"sync"
	"time"
)

type memStore struct {
	mu       sync.RWMutex
	posts    map[string]*model.Post
	comments map[string]*model.Comment
}

func NewMemStore() Store {
	return &memStore{
		posts:    map[string]*model.Post{},
		comments: map[string]*model.Comment{},
	}
}

func (m *memStore) CreatePost(ctx context.Context, post *model.Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.posts[post.ID] = post
	return nil
}

func (m *memStore) GetPost(ctx context.Context, id string) (*model.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	post, ok := m.posts[id]
	if !ok {
		return nil, ErrNotFound
	}

	count := 0
	for _, comment := range m.comments {
		if comment.PostID == post.ID {
			count++
		}
	}
	post.CommentsCount = count

	return post, nil
}

func (m *memStore) ListPosts(ctx context.Context) ([]*model.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	posts := make([]*model.Post, 0, len(m.posts))
	for _, p := range m.posts {
		posts = append(posts, p)
	}

	sort.Slice(posts, func(i, j int) bool { return posts[i].CreatedAt.After(posts[j].CreatedAt) })

	for _, p := range posts {
		count := 0
		for _, comment := range m.comments {
			if comment.PostID == p.ID {
				count++
			}
		}
		p.CommentsCount = count
	}

	return posts, nil
}

func (m *memStore) CloseComments(ctx context.Context, id string, closed bool) (*model.Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	post, ok := m.posts[id]
	if !ok {
		return nil, ErrNotFound
	}

	post.CommentsClosed = closed
	return post, nil
}

func (m *memStore) CreateComment(ctx context.Context, comment *model.Comment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if comment.ParentID != nil && *comment.ParentID == "" {
		comment.ParentID = nil
	}

	if comment.ParentID != nil {
		if parent := m.comments[*comment.ParentID]; parent != nil {
			comment.Depth = parent.Depth + 1
		}
	}

	m.comments[comment.ID] = comment
	return nil
}

func (m *memStore) ListComments(ctx context.Context, postID string, parentID *string, after *string, limit int) (*model.CommentPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var items []*model.Comment
	for _, comment := range m.comments {
		if comment.PostID != postID {
			continue
		}
		// добавляем корневые комментарии
		if parentID == nil {
			items = append(items, comment)
			// вложенные комментарии
		} else {
			if comment.ParentID != nil && *comment.ParentID == *parentID {
				items = append(items, comment)
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	start := 0
	if after != nil && *after != "" {
		if decoded, err := base64.StdEncoding.DecodeString(*after); err == nil {
			prevCursor := string(decoded)
			for i, item := range items {
				cursor := cursorOf(item)
				if cursor == prevCursor {
					start = i + 1
					break
				}
			}
		}
	}

	end := start + limit
	if end > len(items) {
		end = len(items)
	}

	page := items[start:end]
	edges := make([]*model.CommentEdge, 0, len(page))
	for _, comment := range page {
		edges = append(edges, &model.CommentEdge{Cursor: encode(cursorOf(comment)), Node: comment})
	}

	pageInfo := &model.PageInfo{HasNextPage: end < len(items)}
	if len(page) > 0 {
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}

	return &model.CommentPage{Edges: edges, PageInfo: pageInfo}, nil
}

func cursorOf(comment *model.Comment) string {
	return comment.CreatedAt.Format(time.RFC3339Nano) + ":" + comment.ID
}

func encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
