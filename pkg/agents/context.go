package agents

import "context"

// phaseCallbackKey is the context key for storing phase callbacks
type phaseCallbackKey struct{}

// WithPhaseCallback attaches a phase callback to the context
func WithPhaseCallback(ctx context.Context, callback PhaseCallback) context.Context {
	return context.WithValue(ctx, phaseCallbackKey{}, callback)
}

// GetPhaseCallback retrieves a phase callback from the context
func GetPhaseCallback(ctx context.Context) (PhaseCallback, bool) {
	callback, ok := ctx.Value(phaseCallbackKey{}).(PhaseCallback)
	return callback, ok
}
