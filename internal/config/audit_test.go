package config

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestAuditConfigDefaultsAndValidation(t *testing.T) {
	root := t.TempDir()
	doc := "[files]\ndefault_id='work'\nwork.path=" + strconv.Quote(root) + "\n"
	cfg, err := Decode([]byte(doc), Direct)
	if err != nil || !cfg.Audit().Enabled || len(cfg.Audit().RequiredOperations) != 0 {
		t.Fatalf("audit default required and enabled state: %v", err)
	}
	if got := cfg.Audit().Recorder.IntentTimeout; got != 2*time.Second {
		t.Fatalf("default audit intent timeout = %s, want 2s", got)
	}
	for _, settings := range []string{
		"retention = '7 weeks'", "retention = 7", "retention = '0 d'",
		"max_store_size = '3 KiB'", "max_queue_events = 1", "max_queue_size = '1 KiB'",
		"failure_policy = 'unknown'", "failure_policy = 'audit_required'",
		"store_path = 'relative/audit'", "required_operations = ['file.write']",
		"enabled = false\nfailure_policy = 'audit_required'\nrequired_operations=['file.write']",
		"failure_policy='audit_required'\nrequired_operations=['x','x']",
		"failure_policy='audit_required'\nrequired_operations=['x space']",
		"failure_policy='audit_required'\nrequired_operations=['']",
	} {
		if _, err := Decode([]byte(doc+"[audit]\n"+settings+"\n"), Direct); err == nil {
			t.Fatalf("invalid audit settings accepted: %q", settings)
		}
	}
	configPath := filepath.Join(root, defaultFileName)
	otherPath := filepath.Join(root, "another.toml")
	testconfig.Write(t, configPath, []byte(doc))
	testconfig.Write(t, otherPath, []byte(doc+"[audit]\nmax_store_size = '128 MB'\nretention = '7 d'\nrequired_intent_timeout = '750 ms'\n"))
	left, err := Load(Source{Role: Direct, Path: configPath})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Load(Source{Role: Direct, Path: otherPath})
	if err != nil {
		t.Fatal(err)
	}
	if left.Audit().Path != filepath.Join(root, "audit") || right.Audit().Path == left.Audit().Path ||
		left.Audit().Binding == right.Audit().Binding || right.Audit().Store.Bytes != 128000000 {
		t.Fatal("two configuration names did not bind independent stores")
	}
	if got := right.Audit().Recorder.IntentTimeout; got != 750*time.Millisecond {
		t.Fatalf("configured audit intent timeout = %s, want 750ms", got)
	}
}
