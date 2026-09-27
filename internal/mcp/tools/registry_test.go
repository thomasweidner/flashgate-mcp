package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	tool := &testTool{
		name: "test_tool",
	}

	registry.Register(tool)

	got, ok := registry.Get("test_tool")
	if !ok {
		t.Fatal("expected tool to exist")
	}

	if got != tool {
		t.Fatalf("expected registered tool, got %#v", got)
	}
}

func TestRegistryGetUnknownTool(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	got, ok := registry.Get("missing_tool")
	if ok {
		t.Fatal("expected tool not to exist")
	}

	if got != nil {
		t.Fatalf("expected nil tool, got %#v", got)
	}
}

func TestRegistryListReturnsToolsInRegistrationOrder(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	first := &testTool{name: "first"}
	second := &testTool{name: "second"}
	third := &testTool{name: "third"}

	registry.Register(first)
	registry.Register(second)
	registry.Register(third)

	list := registry.List()

	if len(list) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(list))
	}

	expected := []Tool{first, second, third}

	for i, tool := range expected {
		if list[i] != tool {
			t.Fatalf("expected tool %d to be %#v, got %#v", i, tool, list[i])
		}
	}
}

func TestRegistryRegisterOverridesExistingToolWithoutChangingOrder(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	first := &testTool{
		name:        "test_tool",
		description: "first",
	}

	other := &testTool{
		name:        "other_tool",
		description: "other",
	}

	second := &testTool{
		name:        "test_tool",
		description: "second",
	}

	registry.Register(first)
	registry.Register(other)
	registry.Register(second)

	got, ok := registry.Get("test_tool")
	if !ok {
		t.Fatal("expected tool to exist")
	}

	if got != second {
		t.Fatalf("expected second tool, got %#v", got)
	}

	list := registry.List()

	if len(list) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(list))
	}

	if list[0] != second {
		t.Fatalf("expected replacement to keep first position, got %#v", list[0])
	}

	if list[1] != other {
		t.Fatalf("expected other tool to remain second, got %#v", list[1])
	}
}

type testTool struct {
	name        string
	title       string
	description string
	inputSchema any
	result      any
	err         error
	called      int
	arguments   json.RawMessage
}

func (t *testTool) Name() string {
	return t.name
}

func (t *testTool) Title() string {
	return t.title
}

func (t *testTool) Description() string {
	return t.description
}

func (t *testTool) InputSchema() any {
	return t.inputSchema
}

func (t *testTool) Definition() protocol.Tool {
	return protocol.Tool{
		Name:        t.name,
		Title:       t.title,
		Description: t.description,
		InputSchema: t.inputSchema,
	}
}

func (t *testTool) Execute(_ context.Context, arguments json.RawMessage) (any, error) {
	t.called++
	t.arguments = append(json.RawMessage(nil), arguments...)

	if t.err != nil {
		return nil, t.err
	}

	return t.result, nil
}
