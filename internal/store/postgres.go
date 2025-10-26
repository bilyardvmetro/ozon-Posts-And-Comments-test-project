package store

import (
	"PostsAndCommentsMicroservice/graph/model"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgres(dsn string) (Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &PostgresStore{db: db}, nil
}

func (p *PostgresStore) CreatePost(ctx context.Context, post *model.Post) error {
	const q = `insert into posts (id, title, body, author, comments_closed, created_at)
	values ($1, $2, $3, $4, $5, $6)`

	_, err := p.db.ExecContext(ctx, q, post.ID, post.Title, post.Body, post.Author, post.CommentsClosed, post.CreatedAt)
	return err
}

func (p *PostgresStore) GetPost(ctx context.Context, id string) (*model.Post, error) {
	const q = `select id, title, body, author, comments_closed, created_at, 
       (select count(*) from comments c where c.post_id = posts.id) as comments_count from posts where id = $1`

	var res model.Post

	if err := p.db.QueryRowContext(ctx, q, id).Scan(&res.ID, &res.Title, &res.Body, &res.Author, &res.CommentsClosed, &res.CreatedAt, &res.CommentsCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &res, nil
}

func (p *PostgresStore) ListPosts(ctx context.Context) ([]*model.Post, error) {
	const q = `select id, title, body, author, comments_closed, created_at,
       (select count(*) from comments c where c.post_id = posts.id) as comments_count from posts order by created_at desc`

	rows, err := p.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.Post
	for rows.Next() {
		var row model.Post

		if err := rows.Scan(&row.ID, &row.Title, &row.Body, &row.Author, &row.CommentsClosed, &row.CreatedAt, &row.CommentsCount); err != nil {
			return nil, err
		}
		res = append(res, &row)
	}
	return res, rows.Err()
}

func (p *PostgresStore) CloseComments(ctx context.Context, id string, closed bool) (*model.Post, error) {
	const q = `update posts set comments_closed = $2 where id = $1
			  returning id, title, body, author, comments_closed, created_at,
			  (select count(*) from comments c where c.post_id = posts.id) as comments_count`

	var row model.Post
	if err := p.db.QueryRowContext(ctx, q, id, closed).Scan(&row.ID, &row.Title, &row.Body, &row.Author, &row.CommentsClosed, &row.CreatedAt, &row.CommentsCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (p *PostgresStore) CreateComment(ctx context.Context, comment *model.Comment) error {
	if comment.ParentID != nil && *comment.ParentID == "" {
		comment.ParentID = nil
	}

	depth := 0
	if comment.ParentID != nil {
		const getDepth = `select depth from comments where id = $1`
		if err := p.db.QueryRowContext(ctx, getDepth, *comment.ParentID).Scan(&depth); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		depth++
	}

	const q = `insert into comments(id, post_id, parent_id, body, author, depth, created_at) values ($1, $2, $3, $4, $5, $6, $7)`
	_, err := p.db.ExecContext(ctx, q, comment.ID, comment.PostID, comment.ParentID, comment.Body, comment.Author, depth, comment.CreatedAt)
	if err == nil {
		comment.Depth = depth
	}
	return err
}

func (p *PostgresStore) ListComments(ctx context.Context, postID string, parentID *string, after *string, limit int) (*model.CommentPage, error) {
	args := []any{postID}
	where := `c.post_id = $1`

	if parentID != nil && *parentID != "" {
		where += ` and c.parent_id = $2`
		args = append(args, *parentID)
	}

	if after != nil && *after != "" {
		if ts, id, ok := decodeCursor(*after); ok {
			placeholderTs := len(args) + 1
			placeholderId := len(args) + 2
			where += " and (c.created_at > $" + strconv.Itoa(placeholderTs) + " or (c.created_at = $" + strconv.Itoa(placeholderTs) + " and c.id > $" + strconv.Itoa(placeholderId) + "))"
			args = append(args, ts, id)
		}
	}

	q := fmt.Sprintf(`
    select c.id, c.post_id, c.parent_id, c.body, c.author, c.depth, c.created_at
    from comments c where %s order by c.created_at asc, c.id asc limit %d`,
		where, limit)

	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*model.Comment
	for rows.Next() {
		var cm model.Comment
		if err := rows.Scan(&cm.ID, &cm.PostID, &cm.ParentID, &cm.Body, &cm.Author, &cm.Depth, &cm.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, &cm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	edges := make([]*model.CommentEdge, 0, len(items))
	for _, it := range items {
		cur := encodeCursor(it.CreatedAt, it.ID)
		edges = append(edges, &model.CommentEdge{Cursor: cur, Node: it})
	}

	pageInfo := &model.PageInfo{HasNextPage: len(items) == limit}
	if len(edges) > 0 {
		end := edges[len(edges)-1].Cursor
		pageInfo.EndCursor = &end
	}

	return &model.CommentPage{Edges: edges, PageInfo: pageInfo}, nil
}

func encodeCursor(ts time.Time, id string) string {
	raw := ts.Format(time.RFC3339Nano) + ":" + id
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (time.Time, string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, "", false
	}

	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", false
	}

	return ts, parts[1], true
}

func (p *PostgresStore) BatchCommentsCount(ctx context.Context, postIDs []string) (map[string]int, error) {
	if len(postIDs) == 0 {
		return map[string]int{}, nil
	}

	const q = `select post_id, count(*) from comments where post_id = any($1) group by post_id`
	rows, err := p.db.QueryContext(ctx, q, pgArray(postIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int, len(postIDs))
	for _, id := range postIDs {
		out[id] = 0
	}

	for rows.Next() {
		var pid string
		var cnt int
		if err := rows.Scan(&pid, &cnt); err != nil {
			return nil, err
		}
		out[pid] = cnt
	}
	return out, rows.Err()
}

// обертка для pgx
func pgArray(ss []string) any { return ss }
