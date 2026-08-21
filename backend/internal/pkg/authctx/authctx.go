// Package authctx carries the authenticated user ID through a request
// context, so services can attach ownership without new parameters and
// anonymous requests keep working unchanged.
package authctx

import "context"

type key struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, key{}, userID)
}

// UserID returns the authenticated user's ID, if any.
func UserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(key{}).(string)
	return id, ok && id != ""
}
