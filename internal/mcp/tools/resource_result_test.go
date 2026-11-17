package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

func TestWrappedResultSizeMatchesJSON(t *testing.T) {
	for _, s := range []string{"", "abc", "<>\"\\\n\t\x00\u2028\u2029", "ä🙂", "\xff", strings.Repeat("x", 9000)} {
		for _, blocks := range [][]protocol.ContentBlock{
			{protocol.NewTextContent(s)}, {protocol.NewTextContent(s), protocol.NewTextContent(s)},
			{protocol.MediaContent{Type: "image", MIMEType: "image/png", Data: []byte(s)}},
		} {
			for _, isError := range []bool{false, true} {
				result := protocol.CallToolResult{Content: blocks, IsError: isError}
				if !isError {
					result.StructuredContent = json.RawMessage(`{"size":1}`)
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				if !wrappedResultFits(result, int64(len(encoded))) || wrappedResultFits(result, int64(len(encoded)-1)) {
					t.Fatalf("inexact result count: %s", encoded)
				}
			}
		}
	}
	if wrappedResultFits(protocol.CallToolResult{}, -1) || wrappedResultFits(protocol.CallToolResult{Content: []protocol.ContentBlock{nil}}, 1000) {
		t.Fatal("invalid result admitted")
	}
	if wrappedResultFits(protocol.CallToolResult{Content: []protocol.ContentBlock{protocol.TextContent{Type: "other"}}}, 1000) {
		t.Fatal("wrong block type")
	}
}

func resourceFixture() *fakeFileSystem {
	return &fakeFileSystem{entries: []fs.Entry{{Name: "<name>\n", Size: 5}}, readContent: []byte("<text>\n"), statMetadata: fs.Metadata{Name: "<name>", Size: 5}}
}

func TestEveryToolResultLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tool     func(*fakeFileSystem) Tool
		args     string
		mutation bool
	}{
		{"read", func(f *fakeFileSystem) Tool { return NewReadFileTool(f, 1000) }, `{"root_id":"work","path":"x"}`, false},
		{"list", func(f *fakeFileSystem) Tool { return NewListDirectoryTool(f) }, `{"root_id":"work","path":"."}`, false},
		{"stat", func(f *fakeFileSystem) Tool { return NewGetPathInfoTool(f) }, `{"root_id":"work","path":"x"}`, false},
		{"write", func(f *fakeFileSystem) Tool { return NewWriteFileTool(f) }, `{"root_id":"work","path":"x<\n","content":"ok"}`, true},
		{"mkdir", func(f *fakeFileSystem) Tool { return NewCreateDirectoryTool(f) }, `{"root_id":"work","path":"x"}`, true},
		{"delete", func(f *fakeFileSystem) Tool { return NewDeletePathTool(f) }, `{"root_id":"work","path":"x"}`, true},
		{"copy", func(f *fakeFileSystem) Tool { return NewCopyPathTool(f) }, `{"source_root_id":"work","source":"x","target_root_id":"work","target":"y"}`, true},
		{"move", func(f *fakeFileSystem) Tool { return NewMovePathTool(f) }, `{"source_root_id":"work","source":"x","target_root_id":"work","target":"y"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := resourceFixture()
			tool := tc.tool(f)
			result, err := tool.Execute(context.Background(), json.RawMessage(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			wrapped, err := tool.ResultContract().wrap(context.Background(), result)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(wrapped)
			if err != nil {
				t.Fatal(err)
			}
			for _, delta := range []int64{0, -1} {
				f = resourceFixture()
				limits := fs.DefaultLimits().ResourceLimits()
				limits.Result = int64(len(encoded)) + delta
				f.limits = &limits
				_, err = tc.tool(f).Execute(context.Background(), json.RawMessage(tc.args))
				if (err == nil) != (delta == 0) {
					t.Fatalf("delta=%d: %v", delta, err)
				}
				if delta < 0 && tc.mutation && (f.writePath != "" || f.mkdirPath != "" || f.deletePath != "" || f.copySource != "" || f.moveSource != "") {
					t.Fatal("effect occurred on result denial")
				}
			}
			f = resourceFixture()
			ctx := resource.WithResultLimit(context.Background(), 1)
			if _, err := tc.tool(f).Execute(ctx, json.RawMessage(tc.args)); err == nil {
				t.Fatal("global framed allowance bypass")
			}
		})
	}
}

func TestMkdirCachedReceiptMatchesActualOutcome(t *testing.T) {
	for _, created := range []bool{false, true} {
		for _, path := range []string{"x", strings.Repeat("x", 9000)} {
			f := resourceFixture()
			f.mkdirCreated = created
			tool := NewCreateDirectoryTool(f)
			args, _ := json.Marshal(map[string]any{"root_id": "work", "path": path})
			result, err := tool.Execute(context.Background(), args)
			if err != nil {
				t.Fatal(err)
			}
			actual := result.prepared
			result.prepared = nil
			want, err := tool.ResultContract().wrap(context.Background(), result)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(actual)
			w, _ := json.Marshal(want)
			if string(a) != string(w) {
				t.Fatal("cached result differs from outcome")
			}
		}
	}
}

func TestMkdirAdmissionCountMatchesBothOutcomes(t *testing.T) {
	for _, p := range []string{"x", "<>\"\\\n\t\x00\u2028\u2029", "\xff", strings.Repeat("x", 8163), strings.Repeat("x", 8164), strings.Repeat("x", 8165), strings.Repeat("x", 8166), strings.Repeat("x", 9000)} {
		for _, created := range []bool{false, true} {
			result := NewToolResult(defaultVariant, createDirectoryResult{Path: p, Created: created})
			wrapped, err := metadataContract(defaultVariant).wrap(context.Background(), result)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(wrapped)
			if err != nil {
				t.Fatal(err)
			}
			if mkdirReceiptSize(p, created) != int64(len(data)) {
				t.Fatal("mkdir count", len(p), created, mkdirReceiptSize(p, created), len(data))
			}
		}
	}
}
