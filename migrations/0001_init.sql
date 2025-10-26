create table if not exists posts
(
    id              uuid primary key,
    title           text        not null,
    body            text        not null,
    author          text        not null,
    comments_closed boolean     not null default false,
    created_at      timestamptz not null
);

create table if not exists comments
(
    id         uuid primary key,
    post_id    uuid        not null references posts (id) on delete cascade,
    parent_id  uuid        not null references comments (id) on delete cascade,
    body       text        not null check ( char_length(body) <= 2000 ),
    author     text        not null,
    depth      int         not null default 0,
    created_at timestamptz not null
);

create index if not exists idx_comments_post_id_parent_id_time_id
    on comments (post_id, parent_id, created_at, id);

create index if not exists idx_comments_post_id_time_id
    on comments (post_id, created_at, id);
