package config

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

const (
	MaxDocumentBytes = 64 * 1024
	defaultFileName  = "flashgate_config.toml"
	defaultFileBytes = int64(10 * 1024 * 1024)
	defaultWireBytes = int64(16 * 1024 * 1024)
)

// Role is supplied by the host before loading configuration. A TOML file
// cannot select a different role or the trust policy used to open it.
type Role string

const (
	Direct        Role = "stdio"
	Proxy         Role = "proxy"
	Auto          Role = "auto"
	SystemService Role = "service"
	UserHost      Role = "user-host"
)

// Source identifies the single file that the trusted host will open.
// An empty Path selects the operating-system default for Role.
type Source struct {
	Role Role
	Path string
}

// Config is the effective configuration. Its accessors return values, and
// consumers never receive the decoder's mutable destination.
type Config struct {
	data       settings
	role       Role
	sourcePath string
	sourceInfo os.FileInfo
}

type trustedDocument struct {
	data []byte
	info os.FileInfo
}

type settings struct {
	Runtime    runtimeFields    `toml:"runtime"`
	Filesystem filesystemFields `toml:"filesystem"`
	Security   securityFields   `toml:"security"`
	Protocol   protocolFields   `toml:"protocol"`
	Endpoints  endpointFields   `toml:"endpoints"`
	Logging    loggingFields    `toml:"logging"`
}

type runtimeFields struct {
	Mode Role `toml:"mode"`
}

type filesystemFields struct {
	Root             *string `toml:"root"`
	ReadOnly         bool    `toml:"read_only"`
	AllowCWDRoot     bool    `toml:"allow_cwd_root"`
	MaxFileSize      int64   `toml:"max_file_size"`
	MaxWriteBytes    int64   `toml:"max_write_bytes"`
	MaxListEntries   int     `toml:"max_list_entries"`
	MaxCopyBytes     int64   `toml:"max_copy_bytes"`
	MaxDeleteEntries int     `toml:"max_delete_entries"`
}

type securityFields struct {
	AllowHiddenFiles bool `toml:"allow_hidden_files"`
	AllowUNCPaths    bool `toml:"allow_unc_paths"`
	FollowSymlinks   bool `toml:"follow_symlinks"`
}

type protocolFields struct {
	MaxMessageBytes  int64 `toml:"max_message_bytes"`
	MaxArgumentBytes int64 `toml:"max_argument_bytes"`
	MaxResponseBytes int64 `toml:"max_response_bytes"`
}

// Fallback is a policy value for a future auto host. It does not perform discovery.
type Fallback string

const (
	NoFallback     Fallback = "disabled"
	DirectIfAbsent Fallback = "direct-if-absent"
)

type endpointFields struct {
	WindowsPipe   string   `toml:"windows_pipe_name"`
	LinuxSocket   string   `toml:"linux_socket_path"`
	ConnectMS     int64    `toml:"connect_timeout_ms"`
	RequestMS     int64    `toml:"request_timeout_ms"`
	ShutdownMS    int64    `toml:"shutdown_timeout_ms"`
	MaxFrameBytes int64    `toml:"max_frame_bytes"`
	IPCVersion    int      `toml:"ipc_version"`
	Fallback      Fallback `toml:"fallback"`
}

type LogLevel string
type LogSink string
type LogFormat string

const (
	LogOff    LogLevel  = "off"
	LogDebug  LogLevel  = "debug"
	LogStderr LogSink   = "stderr"
	LogText   LogFormat = "text"
	LogJSON   LogFormat = "json"
)

type loggingFields struct {
	Level          LogLevel  `toml:"level"`
	Sink           LogSink   `toml:"sink"`
	Format         LogFormat `toml:"format"`
	MaxRecordBytes int       `toml:"max_record_bytes"`
	MaxRecords     int       `toml:"max_records"`
}

func defaultSettings(role Role) settings {
	return settings{
		Runtime: runtimeFields{Mode: role},
		Filesystem: filesystemFields{
			MaxFileSize: defaultFileBytes, MaxWriteBytes: defaultFileBytes,
			MaxCopyBytes: defaultFileBytes, MaxListEntries: 1000, MaxDeleteEntries: 1000,
		},
		Protocol: protocolFields{
			MaxMessageBytes: defaultWireBytes, MaxArgumentBytes: 12 * 1024 * 1024,
			MaxResponseBytes: defaultWireBytes,
		},
		Endpoints: endpointFields{
			ConnectMS: 5000, RequestMS: 30000, ShutdownMS: 5000,
			MaxFrameBytes: defaultWireBytes, IPCVersion: 1, Fallback: NoFallback,
		},
		Logging: loggingFields{
			Level: LogOff, Sink: LogStderr, Format: LogText, MaxRecordBytes: 4096, MaxRecords: 10000,
		},
	}
}

// DefaultConfig exposes policy defaults. It requires a root before it can be
// used to start the server.
func DefaultConfig() Config {
	return Config{data: defaultSettings(Direct), role: Direct}
}

// Load selects and reads one trusted configuration file, then validates it.
func Load(source Source) (Config, error) {
	return load(source, defaultPath)
}

func load(source Source, locateDefault func(Role) (string, error)) (Config, error) {
	if !validRole(source.Role) {
		return Config{}, NewError(CategoryUnsupportedRuntime, nil)
	}
	path := source.Path
	if path == "" {
		var err error
		path, err = locateDefault(source.Role)
		if err != nil {
			return Config{}, NewError(CategoryUntrustedConfig, err)
		}
	}
	document, err := readTrustedConfig(path, source.Role)
	if err != nil {
		return Config{}, err
	}
	cfg, err := Decode(document.data, source.Role)
	if err != nil {
		return Config{}, err
	}
	cfg.sourcePath = filepath.Clean(path)
	cfg.sourceInfo = document.info
	return cfg, nil
}

// ValidateDataRoot proves that a PathGuard-approved root cannot expose the
// authoritative configuration. Call it for each root before exposing tools.
func (c Config) ValidateDataRoot(guard *security.PathGuard) error {
	if c.sourcePath == "" || c.sourceInfo == nil || guard == nil || guard.ContainsResolvedPath(c.sourcePath) {
		return security.ErrProtectedPath
	}
	if err := checkRootAliases(c.sourcePath, guard); err != nil {
		return err
	}
	if err := guard.RequireOutside(c.sourcePath); err != nil {
		return err
	}
	// Compare the loaded identity on the trusted handle. The native check
	// then samples trust and link count again before closing that handle.
	err := withTrustedConfig(c.sourcePath, c.role, func(file *os.File) error {
		current, err := file.Stat()
		if err != nil {
			return err
		}
		if !os.SameFile(c.sourceInfo, current) {
			return security.ErrProtectedPath
		}
		return nil
	})
	if err != nil {
		return security.ErrProtectedPath
	}
	return nil
}

// Decode parses and validates an already selected configuration document. It
// performs no configuration file I/O and can also validate explicit CLI input.
func Decode(document []byte, role Role) (Config, error) {
	if !validRole(role) {
		return Config{}, NewError(CategoryUnsupportedRuntime, nil)
	}
	if len(document) > MaxDocumentBytes || !utf8.Valid(document) {
		return Config{}, NewError(CategoryInvalidConfig, nil)
	}
	if err := checkTOMLKeys(document); err != nil {
		return Config{}, NewError(CategoryInvalidConfig, err)
	}

	wire := defaultSettings(role)
	decoder := toml.NewDecoder(bytes.NewReader(document)).DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return Config{}, NewError(CategoryInvalidConfig, err)
	}
	cfg := Config{data: wire, role: role}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// The library's struct decoder matches fields case-insensitively even in
// strict mode. Keep the schema's exact lowercase spelling so a case alias
// cannot overwrite a distinct TOML key that targets the same policy field.
func checkTOMLKeys(document []byte) error {
	var parser unstable.Parser
	parser.Reset(document)
	for parser.NextExpression() {
		if !canonicalTOMLKeys(parser.Expression()) {
			return errors.New("noncanonical configuration key")
		}
	}
	return parser.Error()
}

func canonicalTOMLKeys(node *unstable.Node) bool {
	if node.Kind == unstable.Key {
		if len(node.Data) == 0 {
			return false
		}
		for _, ch := range node.Data {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_') {
				return false
			}
		}
	}
	children := node.Children()
	for children.Next() {
		if !canonicalTOMLKeys(children.Node()) {
			return false
		}
	}
	return true
}

// Validate checks the typed settings independently of TOML decoding.
func (c Config) Validate() error {
	if !validRole(c.role) || c.data.Runtime.Mode != c.role {
		return NewError(CategoryUnsupportedRuntime, nil)
	}
	f := c.data.Filesystem
	if f.Root == nil {
		return NewError(CategoryMissingRoot, nil)
	}
	root := *f.Root
	if strings.TrimSpace(root) == "" || strings.IndexByte(root, 0) >= 0 ||
		!filepath.IsAbs(root) && !(root == "." && f.AllowCWDRoot && c.role == Direct) {
		return NewError(CategoryInvalidRoot, nil)
	}
	p := c.data.Protocol
	for _, limit := range []int64{f.MaxFileSize, f.MaxWriteBytes, f.MaxCopyBytes, int64(f.MaxListEntries),
		int64(f.MaxDeleteEntries), p.MaxMessageBytes, p.MaxArgumentBytes, p.MaxResponseBytes} {
		if limit <= 0 || limit == math.MaxInt64 {
			return NewError(CategoryInvalidConfig, errors.New("invalid resource limit"))
		}
	}
	if err := c.data.Endpoints.validate(c.role); err != nil {
		return NewError(CategoryInvalidConfig, err)
	}
	l := c.data.Logging
	if l.Level != LogOff && l.Level != LogDebug ||
		l.Sink != LogStderr || l.Format != LogText && l.Format != LogJSON ||
		l.MaxRecordBytes < 128 || l.MaxRecordBytes > MaxDocumentBytes ||
		l.MaxRecords < 1 || l.MaxRecords > 1000000 {
		return NewError(CategoryInvalidConfig, errors.New("unsupported logging settings"))
	}
	return nil
}

func validRole(role Role) bool {
	switch role {
	case Direct, Proxy, Auto, SystemService, UserHost:
		return true
	}
	return false
}

func (e endpointFields) validate(role Role) error {
	if role == Direct && e != defaultSettings(role).Endpoints {
		return errors.New("endpoint settings require a managed runtime")
	}
	if e.Fallback != NoFallback && !(role == Auto && e.Fallback == DirectIfAbsent) ||
		e.IPCVersion != 1 || e.MaxFrameBytes < 1 || e.MaxFrameBytes > 64*1024*1024 {
		return errors.New("unsupported endpoint policy")
	}
	for _, timeout := range []int64{e.ConnectMS, e.RequestMS, e.ShutdownMS} {
		if timeout < 1 || timeout > int64(time.Hour/time.Millisecond) {
			return errors.New("invalid endpoint timeout")
		}
	}
	for _, ch := range e.WindowsPipe {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' ||
			ch == '-' || ch == '_' || ch == '.') {
			return errors.New("invalid local pipe name")
		}
	}
	if len(e.WindowsPipe) > 128 || e.WindowsPipe == "." || e.WindowsPipe == ".." {
		return errors.New("invalid local pipe name")
	}
	if s := e.LinuxSocket; s != "" && (!path.IsAbs(s) || path.Clean(s) != s || s == "/" ||
		len(s) > 107 || strings.ContainsAny(s, "\x00\r\n\\")) {
		return errors.New("invalid local socket path")
	}
	return nil
}

// FilesystemConfig, SecurityConfig and ServerConfig expose immutable policy values.
type FilesystemConfig struct{ data filesystemFields }
type SecurityConfig struct{ data securityFields }
type ServerConfig struct{ data protocolFields }

// EndpointConfig and LoggingConfig are value snapshots for runtime consumers.
type EndpointConfig struct {
	WindowsPipeName, LinuxSocketPath                string
	ConnectTimeout, RequestTimeout, ShutdownTimeout time.Duration
	MaxFrameBytes                                   int64
	IPCVersion                                      int
	Fallback                                        Fallback
}

type LoggingConfig struct {
	Level                      LogLevel
	Sink                       LogSink
	Format                     LogFormat
	MaxRecordBytes, MaxRecords int
}

func (c Config) Role() Role                   { return c.role }
func (c Config) Filesystem() FilesystemConfig { return FilesystemConfig{c.data.Filesystem} }
func (c Config) Security() SecurityConfig     { return SecurityConfig{c.data.Security} }
func (c Config) Server() ServerConfig         { return ServerConfig{c.data.Protocol} }
func (c Config) Endpoints() EndpointConfig {
	e := c.data.Endpoints
	return EndpointConfig{
		WindowsPipeName: e.WindowsPipe, LinuxSocketPath: e.LinuxSocket,
		ConnectTimeout:  time.Duration(e.ConnectMS) * time.Millisecond,
		RequestTimeout:  time.Duration(e.RequestMS) * time.Millisecond,
		ShutdownTimeout: time.Duration(e.ShutdownMS) * time.Millisecond,
		MaxFrameBytes:   e.MaxFrameBytes, IPCVersion: e.IPCVersion, Fallback: e.Fallback,
	}
}
func (c Config) Logging() LoggingConfig { return LoggingConfig(c.data.Logging) }

func (f FilesystemConfig) RootPath() string {
	if f.data.Root == nil {
		return ""
	}
	return *f.data.Root
}
func (f FilesystemConfig) ReadOnly() bool        { return f.data.ReadOnly }
func (f FilesystemConfig) AllowCWDRoot() bool    { return f.data.AllowCWDRoot }
func (f FilesystemConfig) MaxFileSize() int64    { return f.data.MaxFileSize }
func (f FilesystemConfig) MaxWriteBytes() int64  { return f.data.MaxWriteBytes }
func (f FilesystemConfig) MaxListEntries() int   { return f.data.MaxListEntries }
func (f FilesystemConfig) MaxCopyBytes() int64   { return f.data.MaxCopyBytes }
func (f FilesystemConfig) MaxDeleteEntries() int { return f.data.MaxDeleteEntries }
func (s SecurityConfig) AllowHiddenFiles() bool  { return s.data.AllowHiddenFiles }
func (s SecurityConfig) AllowUNCPaths() bool     { return s.data.AllowUNCPaths }
func (s SecurityConfig) FollowSymlinks() bool    { return s.data.FollowSymlinks }
func (s ServerConfig) Name() string              { return "flashgate" }
func (s ServerConfig) Version() string           { return "0.1.0-dev" }
func (s ServerConfig) MaxMessageBytes() int64    { return s.data.MaxMessageBytes }
func (s ServerConfig) MaxArgumentBytes() int64   { return s.data.MaxArgumentBytes }
func (s ServerConfig) MaxResponseBytes() int64   { return s.data.MaxResponseBytes }

func invalidSourcePath() error { return NewError(CategoryUntrustedConfig, nil) }

func sourceError(err error) error {
	if errors.Is(err, errMissingFile) {
		return NewError(CategoryMissingConfig, err)
	}
	return NewError(CategoryUntrustedConfig, err)
}

var errMissingFile = errors.New("configuration source is absent")

func oversizedDocument() error {
	return NewError(CategoryInvalidConfig, fmt.Errorf("configuration exceeds %d bytes", MaxDocumentBytes))
}
