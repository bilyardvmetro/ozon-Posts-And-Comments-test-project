package logger

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/rs/zerolog/log"
)

func AttachGraphQLHooks(srv *handler.Server) {
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		operation := graphql.GetOperationContext(ctx)

		opType := "unknown"
		if operation.Operation != nil {
			opType = string(operation.Operation.Operation)
		}
		opName := operation.Operation.Name
		varJSON, _ := json.Marshal(operation.Variables)

		Log.Info().
			Str("op_type", opType).
			Str("op_name", opName).
			RawJSON("variables", varJSON).
			Msg("graphql operation started")

		start := time.Now()
		resp := next(ctx)

		return func(ctx context.Context) *graphql.Response {
			Log.Info().
				Str("op_type", opType).
				Str("op_name", opName).
				Dur("duration", time.Since(start)).
				Msg("graphql operation finished")

			return resp(ctx)
		}
	})

	srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (res any, err error) {
		fc := graphql.GetFieldContext(ctx)

		if strings.HasPrefix(fc.Field.Name, "__") ||
			strings.HasPrefix(fc.Object, "__") ||
			strings.HasPrefix(fc.Path().String(), "__") {
			return next(ctx)
		}

		if !fc.IsResolver || (fc.Object != "Query" && fc.Object != "Mutation") {
			return next(ctx)
		}

		start := time.Now()
		argsJSON, _ := json.Marshal(fc.Args)

		Log.Debug().
			Str("field", fc.Field.Name).
			Str("path", fc.Path().String()).
			RawJSON("args", argsJSON).
			Msg("resolver enter")

		res, err = next(ctx)

		event := log.Debug().
			Str("field", fc.Field.Name).
			Str("path", fc.Path().String()).
			Dur("duration", time.Since(start))

		if err != nil {
			event = log.Error().Err(err).
				Str("field", fc.Field.Name).
				Str("path", fc.Path().String()).
				Dur("duration", time.Since(start))
		}

		event.Msg("resolver exit")
		return res, err

	})
}
