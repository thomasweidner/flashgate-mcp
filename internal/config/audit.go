package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/thomasweidner/flashgate-mcp/internal/audit"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

type durationValue time.Duration

func (d *durationValue) UnmarshalTOML(raw []byte) error {
	var scalar struct {
		Duration string `toml:"d"`
	}
	if err := toml.Unmarshal(append([]byte("d = "), raw...), &scalar); err != nil {
		return err
	}
	value, err := resource.ParseDuration(scalar.Duration)
	if err != nil {
		return err
	}
	*d = durationValue(value)
	return nil
}

type AuditMode string

const (
	OperationFirst AuditMode = "operation_first"
	AuditRequired  AuditMode = "audit_required"
)

type auditFields struct {
	Enabled            bool          `toml:"enabled"`
	Mode               AuditMode     `toml:"failure_policy"`
	Path               string        `toml:"store_path"`
	StoreBytes         sizeValue     `toml:"max_store_size"`
	SegmentBytes       sizeValue     `toml:"max_segment_size"`
	Retention          durationValue `toml:"retention"`
	EventBytes         sizeValue     `toml:"max_event_size"`
	QueueEvents        int           `toml:"max_queue_events"`
	QueueBytes         sizeValue     `toml:"max_queue_size"`
	BatchEvents        int           `toml:"max_batch_events"`
	BatchBytes         sizeValue     `toml:"max_batch_size"`
	FlushInterval      durationValue `toml:"flush_interval"`
	Shutdown           durationValue `toml:"flush_timeout"`
	IntentTimeout      durationValue `toml:"required_intent_timeout"`
	RequiredOperations []string      `toml:"required_operations"`
}

func defaultAuditFields() auditFields {
	return auditFields{
		Enabled: true, Mode: OperationFirst, StoreBytes: 256 << 20, SegmentBytes: 16 << 20,
		Retention: durationValue(30 * 24 * time.Hour), EventBytes: 4 << 10, QueueEvents: 1024,
		QueueBytes: 4 << 20, BatchEvents: 128, BatchBytes: 256 << 10,
		FlushInterval: durationValue(100 * time.Millisecond), Shutdown: durationValue(2 * time.Second),
		IntentTimeout: durationValue(2 * time.Second),
	}
}

type AuditConfig struct {
	Enabled            bool
	Path               string
	Binding            string
	PrincipalKey       [32]byte
	Store              audit.StoreLimits
	Recorder           audit.Limits
	RequiredOperations []string
}

func (a auditFields) limits() AuditConfig {
	return AuditConfig{
		Enabled: a.Enabled, Path: a.Path,
		Store: audit.StoreLimits{Bytes: int64(a.StoreBytes), SegmentBytes: int64(a.SegmentBytes), Retention: time.Duration(a.Retention)},
		Recorder: audit.Limits{
			EventBytes: int(a.EventBytes), QueueEvents: a.QueueEvents, QueueBytes: int(a.QueueBytes),
			BatchEvents: a.BatchEvents, BatchBytes: int(a.BatchBytes),
			FlushInterval: time.Duration(a.FlushInterval), Shutdown: time.Duration(a.Shutdown), IntentTimeout: time.Duration(a.IntentTimeout),
		},
		RequiredOperations: slices.Clone(a.RequiredOperations),
	}
}

func (a auditFields) validate() error {
	cfg := a.limits()
	invalid := errors.New("invalid audit settings")
	if !cfg.Recorder.Valid() || !cfg.Store.Valid() || int64(cfg.Recorder.EventBytes)+512 > cfg.Store.SegmentBytes ||
		a.Mode != OperationFirst && a.Mode != AuditRequired ||
		(a.Mode == AuditRequired) != (len(a.RequiredOperations) > 0) ||
		!a.Enabled && a.Mode == AuditRequired || len(a.RequiredOperations) > 128 {
		return invalid
	}
	if a.Path != "" && (len(a.Path) > 4096 || !filepath.IsAbs(a.Path) ||
		filepath.Clean(a.Path) != a.Path || filepath.Dir(a.Path) == a.Path ||
		strings.ContainsAny(a.Path, "\x00\r\n") || runtime.GOOS == "windows" && len(filepath.VolumeName(a.Path)) != 2) {
		return invalid
	}
	seen := make(map[string]bool, len(a.RequiredOperations))
	for _, operation := range a.RequiredOperations {
		if operation == "" || len(operation) > 128 || seen[operation] ||
			strings.IndexFunc(operation, func(ch rune) bool {
				return !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' ||
					ch == '_' || ch == '-' || ch == ':' || ch == '.')
			}) >= 0 {
			return invalid
		}
		seen[operation] = true
	}
	return nil
}

// Audit binds configuration identity to the canonical selected path. Content
// edits leave the store association intact. Other file names under the same
// parent get separate default stores.
func (c Config) Audit() AuditConfig {
	cfg := c.data.Audit.limits()
	if c.sourcePath == "" {
		return cfg
	}
	bound := c.sourceIdentity
	id := sha256.Sum256([]byte(string(c.role) + "\x00" + bound))
	cfg.Binding = hex.EncodeToString(id[:])
	cfg.PrincipalKey = sha256.Sum256([]byte("flashgate-audit-principal-v1\x00" + string(c.role) + "\x00" + bound))
	if cfg.Path == "" {
		name := "audit"
		if filepath.Base(bound) != defaultFileName {
			name += "-" + cfg.Binding
		}
		cfg.Path = filepath.Join(filepath.Dir(c.sourcePath), name)
	}
	return cfg
}
