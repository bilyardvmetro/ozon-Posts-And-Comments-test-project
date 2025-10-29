package logctx

import (
	"context"

	"github.com/rs/zerolog"
)

type key struct{}

var k key

func Into(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, k, l)
}

func From(ctx context.Context, fallback zerolog.Logger) zerolog.Logger {
	if v := ctx.Value(k); v != nil {
		if l, ok := v.(zerolog.Logger); ok {
			return l
		}
	}
	return fallback
}
