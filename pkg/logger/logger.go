package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type contextKey struct{}

// New builds a logger for the given environment.
// "production" → JSON output (structured, machine-readable).
// anything else  → colored console output (human-readable for local dev).
func New(env string) (*zap.Logger, error) {
	if env == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}

// Must panics if logger construction fails.
// Safe to use in main() where there is no recovery path anyway.
func Must(env string) *zap.Logger {
	l, err := New(env)
	if err != nil {
		panic(fmt.Sprintf("failed to init logger: %v", err))
	}
	return l
}

// WithContext stores l in ctx and returns the new context.
// Called once per request by the logger middleware.
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the logger stored by WithContext.
// Returns a no-op logger if none was set — safe fallback, no panic.
func FromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(contextKey{}).(*zap.Logger); ok {
		return l
	}
	return zap.NewNop()
}

// L is shorthand for FromContext.
// Usage in handlers: logger.L(ctx).Info("user registered", zap.String("username", u))
func L(ctx context.Context) *zap.Logger {
	return FromContext(ctx)
}
