package auth

import (
	"context"
	"net/http"
)

// User — минимальная модель текущего пользователя.
type User struct {
	Name string
}

type ctxKey struct{}

var userKey ctxKey

// WithUser извлекает имя из заголовка X-User и кладёт в контекст запроса.
// В проде тут может быть JWT и полноценная валидация.
func WithUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Header.Get("X-User")
		if u == "" {
			// Гость: оставляем nil — некоторые операции можно разрешить и гостю.
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, &User{Name: u})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext возвращает пользователя из контекста (или nil).
func FromContext(ctx context.Context) *User {
	if v := ctx.Value(userKey); v != nil {
		if u, ok := v.(*User); ok {
			return u
		}
	}
	return nil
}
