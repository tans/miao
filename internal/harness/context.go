package harness

import "context"

type runIDContextKey struct{}

// WithRunID lets provider usage receipts point back to their durable run.
func WithRunID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, runIDContextKey{}, id)
}

func RunID(ctx context.Context) string {
	id, _ := ctx.Value(runIDContextKey{}).(string)
	return id
}
