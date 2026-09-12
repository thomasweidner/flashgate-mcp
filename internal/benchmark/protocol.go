package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const protocolVersion = "2025-11-25"

type requestSpec struct {
	Method string
	Params any
}

type notificationSpec struct {
	Method string
	Params any
}

type wireRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type wireNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type wireResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *wireError      `json:"error"`
}

type wireError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type responseRecord struct {
	method        string
	requestBytes  uint64
	responseBytes uint64
	resultBytes   uint64
	result        json.RawMessage
}

func marshalRequest(id int, spec requestSpec) ([]byte, error) {
	encoded, err := json.Marshal(wireRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  spec.Method,
		Params:  spec.Params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", spec.Method, err)
	}
	return append(encoded, '\n'), nil
}

func marshalNotification(spec notificationSpec) ([]byte, error) {
	encoded, err := json.Marshal(wireNotification{
		JSONRPC: "2.0",
		Method:  spec.Method,
		Params:  spec.Params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal %s notification: %w", spec.Method, err)
	}
	return append(encoded, '\n'), nil
}

func writeNotification(writer io.Writer, spec notificationSpec) (uint64, error) {
	encoded, err := marshalNotification(spec)
	if err != nil {
		return 0, err
	}
	written, err := writer.Write(encoded)
	if err != nil {
		return 0, fmt.Errorf("write %s notification: %w", spec.Method, err)
	}
	if written != len(encoded) {
		return 0, fmt.Errorf("write %s notification: short write", spec.Method)
	}
	return uint64(written), nil
}

func decodeResponse(line []byte, expectedID int, method string) (wireResponse, error) {
	var response wireResponse
	if err := json.Unmarshal(bytes.TrimSpace(line), &response); err != nil {
		return wireResponse{}, fmt.Errorf("decode %s response: %w", method, err)
	}
	if response.JSONRPC != "2.0" || response.ID != expectedID {
		return wireResponse{}, fmt.Errorf("invalid %s response envelope", method)
	}
	if response.Error != nil {
		return wireResponse{}, fmt.Errorf("%s failed with JSON-RPC code %d", method, response.Error.Code)
	}
	if len(response.Result) == 0 {
		return wireResponse{}, fmt.Errorf("%s response omitted result", method)
	}
	return response, nil
}

func initializeSpec() requestSpec {
	return requestSpec{
		Method: "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]string{
				"name":    "flashgate-benchmark",
				"version": "v1",
			},
		},
	}
}

func initializedNotificationSpec() notificationSpec {
	return notificationSpec{Method: "notifications/initialized"}
}

func validateInitializeResult(result json.RawMessage) error {
	var payload struct {
		ProtocolVersion string          `json:"protocolVersion"`
		Capabilities    json.RawMessage `json:"capabilities"`
		ServerInfo      *struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return fmt.Errorf("decode initialize result: %w", err)
	}
	if payload.ProtocolVersion != protocolVersion {
		return fmt.Errorf("initialize protocolVersion=%q, want %q", payload.ProtocolVersion, protocolVersion)
	}
	if payload.ServerInfo == nil || payload.ServerInfo.Name == "" || payload.ServerInfo.Version == "" {
		return fmt.Errorf("initialize result omitted complete serverInfo")
	}
	if len(payload.Capabilities) == 0 || bytes.Equal(bytes.TrimSpace(payload.Capabilities), []byte("null")) {
		return fmt.Errorf("initialize result omitted capabilities")
	}
	var capabilities map[string]json.RawMessage
	if err := json.Unmarshal(payload.Capabilities, &capabilities); err != nil || capabilities == nil {
		return fmt.Errorf("initialize result capabilities must be an object")
	}
	return nil
}

func toolsListSpec() requestSpec {
	return requestSpec{Method: "tools/list", Params: map[string]any{}}
}

func toolCallSpec(name string, arguments map[string]any) requestSpec {
	return requestSpec{
		Method: "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	}
}

type workflowDefinition struct {
	name              string
	requests          []requestSpec
	expectedReadBytes *uint64
	expectedEntries   *uint64
}

func exactCounter(value uint64) *uint64 { return &value }

func referenceWorkflows() []workflowDefinition {
	pathChecks := []requestSpec{initializeSpec()}
	reads := []requestSpec{initializeSpec()}
	for index := 0; index < 10; index++ {
		path := fmt.Sprintf("path-checks/entry-%02d.txt", index)
		pathChecks = append(pathChecks, toolCallSpec("get_path_info", map[string]any{"path": path}))
		readPath := fmt.Sprintf("read-files/entry-%02d.txt", index)
		reads = append(reads, toolCallSpec("read_file", map[string]any{"path": readPath}))
	}

	return []workflowDefinition{
		{name: "initialize", requests: []requestSpec{initializeSpec()}},
		{name: "initialize_tools_list", requests: []requestSpec{initializeSpec(), toolsListSpec()}},
		{name: "get_path_info_existing", requests: []requestSpec{initializeSpec(), toolCallSpec("get_path_info", map[string]any{"path": "existing.txt"})}},
		{name: "get_path_info_missing", requests: []requestSpec{initializeSpec(), toolCallSpec("get_path_info", map[string]any{"path": "missing.txt"})}},
		{name: "read_file_small", requests: []requestSpec{initializeSpec(), toolCallSpec("read_file", map[string]any{"path": "small.txt"})}, expectedReadBytes: exactCounter(26)},
		{name: "read_file_64kib", requests: []requestSpec{initializeSpec(), toolCallSpec("read_file", map[string]any{"path": "text-64kib.txt"})}, expectedReadBytes: exactCounter(65536)},
		{name: "list_directory_small", requests: []requestSpec{initializeSpec(), toolCallSpec("list_directory", map[string]any{"path": "small-dir"})}, expectedEntries: exactCounter(3)},
		{name: "list_directory_500_entries", requests: []requestSpec{initializeSpec(), toolCallSpec("list_directory", map[string]any{"path": "large-dir"})}, expectedEntries: exactCounter(500)},
		{name: "multiple_path_checks", requests: pathChecks},
		{name: "multiple_file_reads", requests: reads, expectedReadBytes: exactCounter(260)},
	}
}

func validateWorkflowUsefulOutput(workflow workflowDefinition, counters Counters) error {
	if workflow.expectedReadBytes != nil && counters.ReadBytes != *workflow.expectedReadBytes {
		return fmt.Errorf("read_bytes=%d, want exact useful output %d", counters.ReadBytes, *workflow.expectedReadBytes)
	}
	if workflow.expectedEntries != nil && counters.Entries != *workflow.expectedEntries {
		return fmt.Errorf("entries=%d, want exact useful output %d", counters.Entries, *workflow.expectedEntries)
	}
	return nil
}

func addResultCounters(method string, result json.RawMessage, counters *Counters) error {
	if method != "tools/call" {
		return nil
	}

	counters.Calls++
	var envelope struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil || len(envelope.StructuredContent) == 0 {
		return fmt.Errorf("decode tools/call result counters")
	}

	var shape struct {
		Size    *uint64           `json:"size"`
		Content *string           `json:"content"`
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(envelope.StructuredContent, &shape); err != nil {
		return fmt.Errorf("decode structured result counters: %w", err)
	}
	if shape.Content != nil && shape.Size != nil {
		counters.ReadBytes += *shape.Size
	}
	if shape.Entries != nil {
		counters.Entries += uint64(len(shape.Entries))
	}
	return nil
}

func decodeToolsList(result json.RawMessage) (tools int, schemas int, err error) {
	var payload struct {
		Tools []struct {
			OutputSchema json.RawMessage `json:"outputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return 0, 0, fmt.Errorf("decode tools/list result: %w", err)
	}
	for _, tool := range payload.Tools {
		if len(tool.OutputSchema) > 0 && string(tool.OutputSchema) != "null" {
			schemas++
		}
	}
	return len(payload.Tools), schemas, nil
}
