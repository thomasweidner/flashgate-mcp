// Package wiretrace exposes explicit response operations to a local
// test observer. It has no process-wide state, output, or payload retention.
package wiretrace

import (
	"context"
	"encoding/json"
)

type Operation string

const (
	Encode Operation = "encode"
	Decode Operation = "decode"
	Copy   Operation = "copy"
)

type Event struct {
	Operation Operation
	Bytes     int
}

type observerKey struct{}

func Observe(ctx context.Context, observe func(Event)) context.Context {
	return context.WithValue(ctx, observerKey{}, observe)
}

func record(ctx context.Context, operation Operation, size int) {
	if ctx != nil {
		if observe, ok := ctx.Value(observerKey{}).(func(Event)); ok {
			observe(Event{operation, size})
		}
	}
}

func Marshal(ctx context.Context, value any) ([]byte, error) {
	data, err := json.Marshal(value)
	record(ctx, Encode, len(data))
	return data, err
}

func Unmarshal(ctx context.Context, data []byte, value any) error {
	record(ctx, Decode, len(data))
	return json.Unmarshal(data, value)
}

func Text(ctx context.Context, data []byte) string {
	if len(data) != 0 {
		record(ctx, Copy, len(data))
	}
	return string(data)
}
