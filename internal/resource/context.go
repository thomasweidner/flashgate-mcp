package resource

import (
	"context"
	"math"
)

type resultLimitKey struct{}
type resultOverheadKey struct{}

// WithResultLimit carries the remaining global response allowance after the
// adapter accounts for its framing. It can only tighten a root's result cap.
func WithResultLimit(ctx context.Context, n int64) context.Context {
	return context.WithValue(ctx, resultLimitKey{}, rawResultLimit(ctx, n))
}

func rawResultLimit(ctx context.Context, rootLimit int64) int64 {
	if ctx != nil {
		if n, ok := ctx.Value(resultLimitKey{}).(int64); ok {
			return min(rootLimit, n)
		}
	}
	return rootLimit
}

// WithResultOverhead reserves adapter-owned fields inside the MCP result.
// Additive reservations cannot enlarge either the root or transport allowance.
func WithResultOverhead(ctx context.Context, n int64) context.Context {
	current, _ := ctx.Value(resultOverheadKey{}).(int64)
	if n < 0 || current > math.MaxInt64-n {
		return WithResultLimit(ctx, -1)
	}
	return context.WithValue(ctx, resultOverheadKey{}, current+n)
}

func ResultLimit(ctx context.Context, rootLimit int64) int64 {
	limit := rawResultLimit(ctx, rootLimit)
	if limit < 0 {
		return limit
	}
	if ctx != nil {
		if overhead, ok := ctx.Value(resultOverheadKey{}).(int64); ok {
			if overhead > limit {
				return -1
			}
			return limit - overhead
		}
	}
	return limit
}
