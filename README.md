# Posts and Comments Service

Сервис предоставляет API для создания и просмотра постов, добавления комментариев и управления возможностью
комментирования постов.

---

## Архитектура

### Основные компоненты:

- **Go + gqlgen** — реализация GraphQL-сервера.
- **PostgreSQL** — хранилище данных.
- **GraphQL Schema** — описывает объекты `Post`, `Comment` и доступные операции.
- **Resolver layer** — слой бизнес-логики, который реализует резолверы.
- **HTTP middleware** — извлекает имя пользователя из заголовка `X-User` и помещает его в контекст GraphQL-запроса.
- **Subscriptions** — механизм реального времени для уведомления клиентов о новых комментариях.

---

## Сущности

### **Post**

| Поле             | Тип        | Описание                                   |
|------------------|------------|--------------------------------------------|
| `id`             | `ID!`      | Уникальный идентификатор поста             |
| `title`          | `String!`  | Название поста                             |
| `body`           | `String!`  | Текст поста                                |
| `author`         | `String!`  | Имя автора (берётся из заголовка `X-User`) |
| `commentsClosed` | `Boolean!` | Флаг, запрещающий добавление комментариев  |
| `createdAt`      | `Time!`    | Время создания поста                       |

---

### **Comment**

| Поле        | Тип       | Описание                             |
|-------------|-----------|--------------------------------------|
| `id`        | `ID!`     | Уникальный идентификатор комментария |
| `postId`    | `ID!`     | ID поста, к которому он относится    |
| `parentId`  | `ID!`     | ID родительского комментария         |
| `author`    | `String!` | Имя автора комментария               |
| `body`      | `String!` | Текст комментария                    |
| `depth`     | `Int!`    | Глубина вложенности в посте          |
| `createdAt` | `Time!`   | Время создания комментария           |

---

## Доступные операции

### **Query**

#### `posts: [Post!]!`

Возвращает список всех постов.

```graphql
query {
    posts {
        id
        title
        body
        author
        commentsClosed
        createdAt
        commentsCount
    }
}
```

#### `post(id: ID!): Post`

Возвращает пост по его `postId`.

```graphql
query {
    post(id: <post Id>) {
        id
        title
        body
        author
        commentsClosed
        createdAt
        commentsCount
    }
}
```

#### `comments(postId: ID!, parentId: ID, after: String, first: Int = 20): CommentPage!`

Возвращает все комментарии поста или комментарии поста вложенные в `parentId`: ID


```graphql
query {
    comments (postId: <post Id>, parentId: <comment Id>, after: <cursor string>, first: <comments count>) {
        pageInfo{
            endCursor
            hasNextPage
        }
        edges{
            cursor
            node{
                id
                postID
                parentID
                author
                body
                depth
                createdAt
            }
        }
    }
}
```

### **Mutation**

#### `createPost(title: String!, body: String!, author: String!): Post!`

Создаёт новый пост от имени пользователя из заголовка `author`.

````graphql 
mutation {
    createPost(title: <post title>, body: <post text>, author: <author>) {
        id
        title
        body
        author
        commentsClosed
        createdAt
        commentsCount
    }
}
````

#### `addComment(postId: ID!, parentId: ID, body: String!, author: String!): Comment!`

Добавляет комментарий к существующему посту, если `parentId` пуст или не отправлен.

Добавляет вложеннный комментарий к комментарию с ID `parentId`, если он указан.

Если `commentsClosed == true`, сервер возвращает ошибку `comments are closed for this post`.

````graphql
# Add root comment
mutation {
    addComment(postId: <post Id>, body: <comment text>, author: <comment author>) {
        id
        postID
        parentID
        author
        body
        depth
        createdAt
    }
}
````

````graphql
# Add nested comment
mutation {
    addComment(postId: <post Id>, parentId: <parent comment Id>, body: <comment text>, author: <comment author>) {
        id
        postID
        parentID
        author
        body
        depth
        createdAt
    }
}
````

#### `toggleCommentsClosed(postId: ID!, closed: Boolean!, user: String!): Post!`

Позволяет только автору поста запретить или разрешить комментарии.
Если пользователь не является автором поста — возвращается ошибка:

`forbidden: only post author can toggle comments`

````graphql
mutation {
    toggleCommentsClosed(postId: <post Id>, closed: <true | false>, user: <username>!) {
        id
        postID
        parentID
        author
        body
        depth
        createdAt
    }
}
````

### **Subscription**

`commentAdded(postId: ID!): Comment!`

Позволяет получать новые комментарии в реальном времени:

````graphql
subscription {
    commentAdded(postId: <post Id>) {
        id
        postID
        parentID
        author
        body
        depth
        createdAt
    }
}
````

## Запуск

### Локально

```bash
go run ./cmd/myApi
```

### С помощью Docker compose

```bash
docker-compose up --build -d
```

В логе сервера появится вывод: `INF app/cmd/myApi/main.go:112 > starting server addr=:8080`

В логе БД: `database system is ready to accept connections`


## Выбор хранилища

Перед запуском можно выбрать хранилище: БД или in-memory

- В файле [.env](.env) установите property `STORE=pg`, для выбора БД хранилища.
- Для in-memory хранилища оставить property пустой
