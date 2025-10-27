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
| `title`          | `String!`  | Текст поста                                |
| `body`           | `String!`  | Текст поста                                |
| `author`         | `String!`  | Имя автора (берётся из заголовка `X-User`) |
| `commentsClosed` | `Boolean!` | Флаг, запрещающий добавление комментариев  |
| `createdAt`      | `Time!`    | Время создания                             |

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
        author
        body
        createdAt
        commentsClosed
    }
}
```

#### `post(id: ID!): Post`

Возвращает один пост и все его комментарии.

```graphql
query {
    post(id: "123e4567-e89b-12d3-a456-426614174000") {
        id
        author
        body
        commentsClosed
        comments {
            id
            author
            body
            createdAt
        }
    }
}
```

`comments(
        postId: ID!
        parentId: ID
        after: String
        first: Int = 20
    ): CommentPage!`

Возвращает все комментарии поста или комментарии поста вложенные в parentId: ID

### **Mutation**

`createPost(title: String!, body: String!, author: String!): Post!`

Создаёт новый пост от имени пользователя из заголовка X-User.

````graphql 
mutation {
    createPost(body: "Hello, GraphQL!") {
        id
        author
        body
    }
}
````

`    addComment(
        postId: ID!,
        parentId: ID,
        body: String!,
        author: String!
    ): Comment!`

Добавляет комментарий к существующему посту.
Если commentsClosed == true, сервер возвращает ошибку `comments are closed for this post`.

````graphql
mutation {
    addComment(postId: "123e4567-e89b-12d3-a456-426614174000", parentId: <Optional>, body: "Nice post!", author: "bob") {
        id
        author
        body
    }
}
````

`toggleCommentsClosed(postId: ID!, closed: Boolean!): Post!`

Позволяет только автору поста запретить или разрешить комментарии.
Если пользователь не является автором поста — возвращается ошибка:

`forbidden: only post author can toggle comments`

````graphql
mutation {
    toggleCommentsClosed(postId: "123e4567-e89b-12d3-a456-426614174000", closed: true) {
        id
        author
        commentsClosed
    }
}
````

### **Subscription**

`commentAdded(postId: ID!): Comment!`

Позволяет получать новые комментарии в реальном времени:

````graphql
subscription {
    commentAdded(postId: "123e4567-e89b-12d3-a456-426614174000") {
        id
        author
        body
    }
}
````

### Авторизация и контекст

Каждый HTTP-запрос должен содержать заголовок:

`X-User: <username>`

Middleware добавляет `username` в `context.Context`.

Все мутации используют контекст для определения текущего пользователя.

#### Пример HTTP-запроса

```http request
POST http://localhost:8080/query
Content-Type: application/json
X-User: alice

{
"query": "mutation { toggleCommentsClosed(postId: \"76867879-88b9-4f33-a14c-ebda0cf659ba\", closed: true) { id author body createdAt } }"
}
```

## Запуск

### Локально

```bash
go run ./cmd/myApi
```

В консоли появится вывод: `listening on :8080...`

### С помощью Docker compose

```bash
docker-compose up --build -d
```

В логе сервера появится вывод: `listening on :8080...`

В логе БД: `database system is ready to accept connections`


## Выбор хранилища

Перед запуском можно выбрать хранилище: БД или in-memory

- В файле [.env](.env) установите проперти `STORE=pg`, для выбора БД хранилища.
- Для in-memory хранилища оставить проперти пустой
