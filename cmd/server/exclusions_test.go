package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestFilesystemExclusionSTDIO(t *testing.T) {
	root := t.TempDir()
	const excluded = "omitted-entry-375"
	const content = "omitted-payload-375"
	const pattern = "omitted-entry-37?"
	secret := filepath.Join(root, excluded)
	testconfig.Write(t, secret, []byte(content))
	sibling := filepath.Join(root, "sibling.txt")
	testconfig.Write(t, sibling, []byte("ordinary payload"))
	source := filepath.Join(t.TempDir(), "active.toml")
	testconfig.Write(t, source, []byte(configDocument(t, root,
		"work.access=['files.read','files.write']\nwork.excluded=['"+pattern+"']\n"+profileSettings(false))))
	calls := []string{
		`"name":"list_directory","arguments":{"root_id":"work","path":"."}`,
		`"name":"get_path_info","arguments":{"root_id":"work","path":"` + excluded + `"}`,
		`"name":"read_file","arguments":{"root_id":"work","path":"` + excluded + `"}`,
		`"name":"write_file","arguments":{"root_id":"work","path":"` + excluded + `","content":"changed","overwrite":true}`,
		`"name":"create_directory","arguments":{"root_id":"work","path":"omitted-entry-379/descendant"}`,
		`"name":"delete_path","arguments":{"root_id":"work","path":"` + excluded + `"}`,
		`"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"` + excluded + `","target":"new.txt"}`,
		`"name":"move_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"sibling.txt","target":"` + excluded + `","overwrite":true}`,
		`"name":"list_directory","arguments":{"root_id":"work","path":"omitted-entry-378"}`,
	}
	var input, output, diagnostics bytes.Buffer
	for i, call := range calls {
		fmt.Fprintf(&input, `{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{%s}}`+"\n", i+1, call)
	}
	if err := runWithIO(context.Background(), &input, &output, &diagnostics, defaultBootstrapDependencies(source)); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != len(calls) || diagnostics.Len() != 0 {
		t.Fatalf("unexpected STDIO framing: %d responses, %d diagnostic bytes", len(lines), diagnostics.Len())
	}
	for i, line := range lines {
		assertNoHostPathLeak(t, line)
		for _, private := range []string{excluded, content, pattern, "omitted-entry-378", "omitted-entry-379", "descendant"} {
			if strings.Contains(line, private) {
				t.Fatalf("private data in response %d", i)
			}
		}
		var response callToolWireResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil || response.Error != nil {
			t.Fatalf("invalid response %d: %v", i, err)
		}
		decode := mcptest.DecodeCallToolResult
		if i == 0 {
			decode = mcptest.DecodeSplitCallToolResult
		}
		decoded, err := decode(response.Result)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if decoded.IsError || !strings.Contains(line, "sibling.txt") {
				t.Fatal("listing lost the allowed sibling")
			}
		} else if !decoded.IsError || decoded.HasStructuredContent || decoded.TextContent.(map[string]any)["category"] != "access_denied" {
			t.Fatalf("incorrect redaction for response %d: %s", i, line)
		}
	}
	for path, want := range map[string]string{secret: content, sibling: "ordinary payload"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("a denied effect changed an operand: %v", err)
		}
	}
}
