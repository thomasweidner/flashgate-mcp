package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type resultTestTool struct {
	testTool
	contract ResultContract
	output   ToolResult
}

type untrustedResultObject struct{ payload string }

func (untrustedResultObject) resultObject() {}

func (untrustedResultObject) MarshalJSON() ([]byte, error) {
	panic("untrusted objects must be rejected before serialization")
}

func requireTextPart(t *testing.T, text string) ResultPart {
	t.Helper()
	part, err := TextPart(text)
	if err != nil {
		t.Fatal(err)
	}
	return part
}

func (tool *resultTestTool) ResultContract() ResultContract { return tool.contract }

func (tool *resultTestTool) Execute(context.Context, json.RawMessage) (ToolResult, error) {
	return tool.output, nil
}

func TestResultContractRegistrationRejectsInvalidReplacement(t *testing.T) {
	registry := NewRegistry()
	original := &testTool{name: "test_tool"}
	requireRegistration(t, registry, original)

	for _, variant := range []ResultVariant{
		{},
		{Name: "complete", ObjectKind: TextPayload, Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: 100},
		{Name: "complete", ObjectKind: MetadataPayload, Mode: ResourceTransfer, State: CompleteResult, MaxObjectBytes: 100},
		{Name: "complete", ObjectKind: MetadataPayload, Mode: InlineTransfer, State: PendingResult, MaxObjectBytes: 100},
		{Name: "complete", ObjectKind: MetadataPayload, Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: maxMetadataBytes + 1},
		{Name: "complete", ObjectKind: MetadataPayload, Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: 100, Parts: []PayloadKind{TextPayload}},
		{Name: "complete", ObjectKind: MetadataPayload, Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: 100, MaxPayloadBytes: 1, Parts: []PayloadKind{StructuredPayload}},
		{Name: "complete", ObjectKind: StructuredPayload, Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: 100, MaxPayloadBytes: 1, Parts: []PayloadKind{TextPayload}},
	} {
		invalid := &resultTestTool{testTool: testTool{name: "test_tool"}, contract: ResultContract{variant}}
		if err := registry.Register(invalid); err == nil {
			t.Fatalf("invalid variant registered: %#v", variant)
		}
		if current, ok := registry.Get("test_tool"); !ok || current != original {
			t.Fatal("failed registration displaced the valid tool")
		}
	}
	for _, contract := range []ResultContract{nil, metadataContract("complete", "complete")} {
		invalid := &resultTestTool{testTool: testTool{name: "other"}, contract: contract}
		if err := registry.Register(invalid); err == nil {
			t.Fatalf("invalid contract registered: %#v", contract)
		}
	}
}

func TestResultAdapterMultipartWireAndFrozenContract(t *testing.T) {
	contract := ResultContract{{
		Name: "multipart", ObjectKind: MetadataPayload, Parts: []PayloadKind{TextPayload, BinaryPayload},
		Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: 64, MaxPayloadBytes: 6,
	}}
	tool := &resultTestTool{
		testTool: testTool{name: "test_tool"},
		contract: contract,
		output: NewPayloadResult("multipart",
			requireTextPart(t, "é\n"), MediaPart("image/png", []byte{1, 2, 3})),
	}
	registry := NewRegistry()
	requireRegistration(t, registry, tool)
	contract[0].MaxPayloadBytes = 1
	contract[0].Parts[0] = BinaryPayload

	result, rpcErr := NewCallHandler(registry).Handle(handlers.Context{}, json.RawMessage(`{"name":"test_tool"}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"content":[{"type":"text","text":"é\n"},{"type":"image","mimeType":"image/png","data":"AQID"}],"structuredContent":{"size":6}}`
	if !bytes.Equal(wire, []byte(want)) {
		t.Fatalf("unexpected multipart result: %s", wire)
	}
}

func TestResultAdapterRejectsInvalidAndOverLimitResults(t *testing.T) {
	contract := ResultContract{{
		Name: "bounded", ObjectKind: MetadataPayload, Parts: []PayloadKind{TextPayload, BinaryPayload},
		Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: 16, MaxPayloadBytes: 16,
	}}
	for _, tc := range []struct {
		name   string
		result ToolResult
		limit  bool
	}{
		{"unknown variant", NewToolResult("undeclared", createDirectoryResult{}), false},
		{"missing part", NewPayloadResult("bounded", requireTextPart(t, "safe")), false},
		{"wrong part kind", NewPayloadResult("bounded", requireTextPart(t, "a"), requireTextPart(t, "b")), false},
		{"invalid text without constructor", NewPayloadResult("bounded", ResultPart{kind: TextPayload, text: "private\xff"}, MediaPart("audio/wav", []byte{1})), false},
		{"text with binary data", NewPayloadResult("bounded", ResultPart{kind: TextPayload, text: "a", data: []byte("private")}, MediaPart("audio/wav", []byte{1})), false},
		{"text with media type", NewPayloadResult("bounded", ResultPart{kind: TextPayload, text: "a", mime: "image/png"}, MediaPart("audio/wav", []byte{1})), false},
		{"text with empty binary field", NewPayloadResult("bounded", ResultPart{kind: TextPayload, text: "a", data: []byte{}}, MediaPart("audio/wav", []byte{1})), false},
		{"media with text data", NewPayloadResult("bounded", requireTextPart(t, "a"), ResultPart{kind: BinaryPayload, text: "private", mime: "audio/wav", data: []byte{1}}), false},
		{"invalid media", NewPayloadResult("bounded", requireTextPart(t, "a"), MediaPart("application/octet-stream", []byte{1})), false},
		{"aggregate limit", NewPayloadResult("bounded", requireTextPart(t, "sensitive-marker"), MediaPart("audio/wav", []byte{1})), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistry()
			requireRegistration(t, registry, &resultTestTool{
				testTool: testTool{name: "test_tool"}, contract: contract, output: tc.result,
			})
			value, rpcErr := NewCallHandler(registry).Handle(handlers.Context{}, json.RawMessage(`{"name":"test_tool"}`))
			if !tc.limit {
				if value != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInternalError || rpcErr.Message != "internal error" || rpcErr.Data != nil {
					t.Fatalf("expected generic internal error: %#v, %#v", value, rpcErr)
				}
				return
			}
			wire, err := json.Marshal(value)
			if err != nil || rpcErr != nil {
				t.Fatalf("expected tool limit error: %v, %v", err, rpcErr)
			}
			decoded, err := mcptest.DecodeCallToolResult(wire)
			if err != nil || !decoded.IsError || decoded.HasStructuredContent ||
				decoded.TextContent.(map[string]any)["category"] != "limit_exceeded" ||
				bytes.Contains(wire, []byte("sensitive-marker")) {
				t.Fatalf("unsafe limit result: %s, %v", wire, err)
			}
		})
	}
}

func TestOversizeReceiptRejectedBeforeWrite(t *testing.T) {
	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)
	raw, err := json.Marshal(map[string]string{"path": strings.Repeat("./", maxMetadataBytes) + "target"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tool.Execute(context.Background(), raw)
	if errorCategory(err) != "limit_exceeded" || filesystem.writePath != "" {
		t.Fatalf("over-limit receipt reached the filesystem: %v", err)
	}
}

func TestMetadataParityFallsBackToSummary(t *testing.T) {
	object := createDirectoryResult{Path: strings.Repeat("x", metadataParityBytes), Created: true}
	result, err := metadataContract(defaultVariant).wrap(NewToolResult(defaultVariant, object))
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mcptest.DecodeSplitCallToolResult(wire)
	if err != nil || decoded.TextContent != "See structuredContent for the result." ||
		decoded.StructuredContent.(map[string]any)["path"] != object.Path {
		t.Fatalf("unexpected bounded metadata fallback: %v", err)
	}
}

func TestPayloadResultsRejectDuplicateTextAndMediaInCallerObjects(t *testing.T) {
	text := strings.Repeat("payload-text-<>&", 4096)
	media := bytes.Repeat([]byte{0, 1, 2}, 16*1024)
	for _, tc := range []struct {
		name   string
		part   ResultPart
		object resultObject
	}{
		{"text copied into receipt", requireTextPart(t, text), createDirectoryResult{Path: text}},
		{"media copied into custom object", MediaPart("image/png", media), untrustedResultObject{payload: string(media)}},
		{"media copied as base64 in receipt", MediaPart("image/png", media), createDirectoryResult{Path: strings.Repeat("AAEC", 16*1024)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contract := ResultContract{{
				Name: defaultVariant, ObjectKind: MetadataPayload, Mode: InlineTransfer, State: CompleteResult,
				Parts: []PayloadKind{tc.part.kind}, MaxObjectBytes: maxMetadataBytes, MaxPayloadBytes: maxMetadataBytes,
			}}
			registry := NewRegistry()
			requireRegistration(t, registry, &resultTestTool{
				testTool: testTool{name: "test_tool"}, contract: contract,
				output: ToolResult{variant: defaultVariant, object: tc.object, parts: []ResultPart{tc.part}},
			})
			result, rpcErr := NewCallHandler(registry).Handle(handlers.Context{}, json.RawMessage(`{"name":"test_tool"}`))
			if result != nil || rpcErr == nil || rpcErr.Message != "internal error" {
				t.Fatalf("mixed payload and caller metadata escaped: %v, %#v", result != nil, rpcErr)
			}
		})
	}
}

func TestResultObjectLimitsMatchEncodedSize(t *testing.T) {
	escaped := "a<>&\"\\\x00\b\f\r\t\n\u2028\u2029ä💾\ufffd\xff"
	for _, object := range []resultObject{
		getPathInfoExistingResult{Path: escaped, Name: escaped, Exists: true, IsDir: true, Size: math.MinInt64},
		getPathInfoMissingResult{Path: escaped},
		writeFileResult{Path: escaped, Size: math.MaxInt64, Written: true},
		createDirectoryResult{Path: escaped, Created: true},
		deletePathResult{Path: escaped},
		copyPathResult{Source: escaped, Target: escaped},
		movePathResult{Source: escaped, Target: escaped, Moved: true},
		listDirectoryResult{},
		listDirectoryResult{Entries: []fs.Entry{}},
		listDirectoryResult{Entries: []fs.Entry{{Name: escaped, IsDir: true, Size: 42}, {Name: "other"}}},
	} {
		encoded, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		if fits, err := resultObjectFits(object, len(encoded)); !fits || err != nil {
			t.Fatalf("%T rejected at exact encoded limit: %v", object, err)
		}
		if fits, err := resultObjectFits(object, len(encoded)-1); fits || err != nil {
			t.Fatalf("%T exceeded the pre-serialization limit: %v", object, err)
		}
	}
}

func TestOversizeObjectsAndDerivedMetadataFailClosed(t *testing.T) {
	for _, tc := range []struct {
		kind   PayloadKind
		object resultObject
	}{
		{MetadataPayload, createDirectoryResult{Path: strings.Repeat("x", maxMetadataBytes+1)}},
		{StructuredPayload, listDirectoryResult{Entries: []fs.Entry{{Name: strings.Repeat("<", 48)}}}},
		{StructuredPayload, listDirectoryResult{Entries: make([]fs.Entry, 1000)}},
	} {
		if encoded, err := encodeResultObject(tc.object, tc.kind, 96); encoded != nil || err != errToolResultLimit {
			t.Fatalf("%T escaped object bound: encoded=%v error=%v", tc.object, encoded != nil, err)
		}
	}

	variant := ResultVariant{
		Name: defaultVariant, ObjectKind: MetadataPayload, Parts: []PayloadKind{TextPayload},
		MaxObjectBytes: len(`{"size":0}`) - 1, MaxPayloadBytes: 16,
	}
	result, err := variant.wrap(NewPayloadResult(defaultVariant, requireTextPart(t, "")))
	if result.Content != nil || err != errToolResultLimit {
		t.Fatalf("derived metadata escaped bound: %v, %v", result.Content, err)
	}
}

func TestOversizeDirectoryResultReturnsOnlyLimitError(t *testing.T) {
	filesystem := newFakeFileSystem()
	// The raw entry is below the cap, but its JSON escapes exceed it.
	filesystem.entries = []fs.Entry{{Name: strings.Repeat("<", maxStructuredBytes/6+1)}}
	registry := NewRegistry()
	requireRegistration(t, registry, NewListDirectoryTool(filesystem))
	result, rpcErr := NewCallHandler(registry).Handle(handlers.Context{}, json.RawMessage(`{"name":"list_directory"}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mcptest.DecodeCallToolResult(wire)
	if err != nil || !decoded.IsError || decoded.HasStructuredContent ||
		decoded.TextContent.(map[string]any)["category"] != "limit_exceeded" {
		t.Fatalf("oversized collection leaked or did not fail closed: %v", err)
	}
}
