package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	envRootPath         = "MCP_ROOT"
	envReadOnly         = "MCP_READ_ONLY"
	envAllowCWDRoot     = "MCP_ALLOW_CWD_ROOT"
	envMaxFileSize      = "MCP_MAX_FILE_SIZE"
	envMaxWriteBytes    = "MCP_MAX_WRITE_BYTES"
	envMaxListEntries   = "MCP_MAX_LIST_ENTRIES"
	envMaxCopyBytes     = "MCP_MAX_COPY_BYTES"
	envMaxCopyEntries   = "MCP_MAX_COPY_ENTRIES"
	envMaxDeleteEntries = "MCP_MAX_DELETE_ENTRIES"
	envAllowHiddenFiles = "MCP_ALLOW_HIDDEN_FILES"
	envAllowUNCPaths    = "MCP_ALLOW_UNC_PATHS"
	envFollowSymlinks   = "MCP_FOLLOW_SYMLINKS"
	envServerDebug      = "MCP_DEBUG"
	envMaxMessageBytes  = "MCP_MAX_JSONRPC_MESSAGE_BYTES"
	envMaxArgumentBytes = "MCP_MAX_TOOL_ARGUMENT_BYTES"
	envMaxResponseBytes = "MCP_MAX_RESPONSE_BYTES"

	defaultRootPath         = ""
	defaultMaxFileSize      = int64(10 * 1024 * 1024) // 10 MiB
	defaultMaxWriteBytes    = int64(10 * 1024 * 1024) // 10 MiB
	defaultMaxListEntries   = 1000
	defaultMaxCopyBytes     = int64(10 * 1024 * 1024) // 10 MiB
	defaultMaxCopyEntries   = 1000
	defaultMaxDeleteEntries = 1000
	defaultServerName       = "flashgate"
	defaultVersion          = "0.1.0-dev"
	defaultMaxMessageBytes  = int64(16 * 1024 * 1024) // 16 MiB
	defaultMaxArgumentBytes = int64(12 * 1024 * 1024) // 12 MiB
	defaultMaxResponseBytes = int64(16 * 1024 * 1024) // 16 MiB
)

// Config contains the complete application configuration.
type Config struct {
	filesystem FilesystemConfig
	security   SecurityConfig
	server     ServerConfig
}

// FilesystemConfig contains filesystem-related configuration.
type FilesystemConfig struct {
	rootPath         string
	readOnly         bool
	allowCWDRoot     bool
	maxFileSize      int64
	maxWriteBytes    int64
	maxListEntries   int
	maxCopyBytes     int64
	maxCopyEntries   int
	maxDeleteEntries int
}

// SecurityConfig contains security-related configuration.
type SecurityConfig struct {
	allowHiddenFiles bool
	allowUNCPaths    bool
	followSymlinks   bool
}

// ServerConfig contains server-related configuration.
type ServerConfig struct {
	name             string
	version          string
	debug            bool
	maxMessageBytes  int64
	maxArgumentBytes int64
	maxResponseBytes int64
}

// DefaultConfig returns the default application configuration.
func DefaultConfig() Config {
	return Config{
		filesystem: FilesystemConfig{
			rootPath:         defaultRootPath,
			readOnly:         false,
			allowCWDRoot:     false,
			maxFileSize:      defaultMaxFileSize,
			maxWriteBytes:    defaultMaxWriteBytes,
			maxListEntries:   defaultMaxListEntries,
			maxCopyBytes:     defaultMaxCopyBytes,
			maxCopyEntries:   defaultMaxCopyEntries,
			maxDeleteEntries: defaultMaxDeleteEntries,
		},
		security: SecurityConfig{
			allowHiddenFiles: false,
			allowUNCPaths:    false,
			followSymlinks:   false,
		},
		server: ServerConfig{
			name:             defaultServerName,
			version:          defaultVersion,
			debug:            false,
			maxMessageBytes:  defaultMaxMessageBytes,
			maxArgumentBytes: defaultMaxArgumentBytes,
			maxResponseBytes: defaultMaxResponseBytes,
		},
	}
}

// LoadFromEnvironment loads configuration from environment variables.
func LoadFromEnvironment() (Config, error) {
	cfg := DefaultConfig()

	rootPath, rootConfigured := os.LookupEnv(envRootPath)
	if !rootConfigured {
		return Config{}, NewError(CategoryMissingRoot, nil)
	}
	cfg.filesystem.rootPath = rootPath

	if value, configured := os.LookupEnv(envReadOnly); configured {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, NewError(CategoryInvalidProfile, err)
		}
		cfg.filesystem.readOnly = parsed
	}

	if value, configured := os.LookupEnv(envAllowCWDRoot); configured {
		switch value {
		case "true":
			cfg.filesystem.allowCWDRoot = true
		case "false":
			cfg.filesystem.allowCWDRoot = false
		default:
			return Config{}, NewError(CategoryInvalidDevelopmentOption, nil)
		}
	}

	if value := os.Getenv(envMaxFileSize); value != "" {
		parsed, err := parsePositiveInt64(value, envMaxFileSize)
		if err != nil {
			return Config{}, err
		}
		cfg.filesystem.maxFileSize = parsed
	}

	if value := os.Getenv(envMaxWriteBytes); value != "" {
		parsed, err := parsePositiveInt64(value, envMaxWriteBytes)
		if err != nil {
			return Config{}, err
		}
		cfg.filesystem.maxWriteBytes = parsed
	}

	if value := os.Getenv(envMaxListEntries); value != "" {
		parsed, err := parsePositiveInt(value, envMaxListEntries)
		if err != nil {
			return Config{}, err
		}
		cfg.filesystem.maxListEntries = parsed
	}

	if value := os.Getenv(envMaxCopyBytes); value != "" {
		parsed, err := parsePositiveInt64(value, envMaxCopyBytes)
		if err != nil {
			return Config{}, err
		}
		cfg.filesystem.maxCopyBytes = parsed
	}

	if value := os.Getenv(envMaxCopyEntries); value != "" {
		parsed, err := parsePositiveInt(value, envMaxCopyEntries)
		if err != nil {
			return Config{}, err
		}
		cfg.filesystem.maxCopyEntries = parsed
	}

	if value := os.Getenv(envMaxDeleteEntries); value != "" {
		parsed, err := parsePositiveInt(value, envMaxDeleteEntries)
		if err != nil {
			return Config{}, err
		}
		cfg.filesystem.maxDeleteEntries = parsed
	}

	if value := os.Getenv(envAllowHiddenFiles); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_ALLOW_HIDDEN_FILES value")
		}
		cfg.security.allowHiddenFiles = parsed
	}

	if value := os.Getenv(envAllowUNCPaths); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_ALLOW_UNC_PATHS value")
		}
		cfg.security.allowUNCPaths = parsed
	}

	if value := os.Getenv(envFollowSymlinks); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_FOLLOW_SYMLINKS value")
		}
		cfg.security.followSymlinks = parsed
	}

	if value := os.Getenv(envServerDebug); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_DEBUG value")
		}
		cfg.server.debug = parsed
	}

	if value := os.Getenv(envMaxMessageBytes); value != "" {
		parsed, err := parsePositiveInt64(value, envMaxMessageBytes)
		if err != nil {
			return Config{}, err
		}
		cfg.server.maxMessageBytes = parsed
	}

	if value := os.Getenv(envMaxArgumentBytes); value != "" {
		parsed, err := parsePositiveInt64(value, envMaxArgumentBytes)
		if err != nil {
			return Config{}, err
		}
		cfg.server.maxArgumentBytes = parsed
	}

	if value := os.Getenv(envMaxResponseBytes); value != "" {
		parsed, err := parsePositiveInt64(value, envMaxResponseBytes)
		if err != nil {
			return Config{}, err
		}
		cfg.server.maxResponseBytes = parsed
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate validates the complete configuration.
func (c Config) Validate() error {
	if strings.TrimSpace(c.filesystem.rootPath) == "" {
		return NewError(CategoryInvalidRoot, nil)
	}

	if c.filesystem.maxFileSize <= 0 {
		return errors.New("maximum file size must be greater than zero")
	}

	if c.filesystem.maxWriteBytes <= 0 {
		return errors.New("maximum write size must be greater than zero")
	}

	if c.filesystem.maxListEntries <= 0 {
		return errors.New("maximum list entries must be greater than zero")
	}

	if c.filesystem.maxCopyBytes <= 0 {
		return errors.New("maximum copy size must be greater than zero")
	}

	if c.filesystem.maxCopyEntries <= 0 {
		return errors.New("maximum copy entries must be greater than zero")
	}

	if c.filesystem.maxDeleteEntries <= 0 {
		return errors.New("maximum delete entries must be greater than zero")
	}

	if c.server.maxMessageBytes <= 0 {
		return errors.New("maximum JSON-RPC message size must be greater than zero")
	}

	if c.server.maxArgumentBytes <= 0 {
		return errors.New("maximum tool argument size must be greater than zero")
	}

	if c.server.maxResponseBytes <= 0 {
		return errors.New("maximum JSON-RPC response size must be greater than zero")
	}

	if filepath.IsAbs(c.filesystem.rootPath) {
		return nil
	}

	if c.filesystem.rootPath == "." && c.filesystem.allowCWDRoot {
		return nil
	}

	return NewError(CategoryInvalidRoot, nil)
}

func parsePositiveInt64(value string, name string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid " + name + " value")
	}

	return parsed, nil
}

func parsePositiveInt(value string, name string) (int, error) {
	parsed, err := strconv.ParseInt(value, 10, 0)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid " + name + " value")
	}

	return int(parsed), nil
}

// Filesystem returns the filesystem configuration.
func (c Config) Filesystem() FilesystemConfig {
	return c.filesystem
}

// Security returns the security configuration.
func (c Config) Security() SecurityConfig {
	return c.security
}

// Server returns the server configuration.
func (c Config) Server() ServerConfig {
	return c.server
}

// RootPath returns the configured filesystem root path.
func (c FilesystemConfig) RootPath() string {
	return c.rootPath
}

// ReadOnly returns whether filesystem writes are disabled.
func (c FilesystemConfig) ReadOnly() bool {
	return c.readOnly
}

// AllowCWDRoot returns whether an explicit MCP_ROOT=. development root is enabled.
func (c FilesystemConfig) AllowCWDRoot() bool {
	return c.allowCWDRoot
}

// MaxFileSize returns the maximum allowed file size in bytes.
func (c FilesystemConfig) MaxFileSize() int64 {
	return c.maxFileSize
}

// MaxWriteBytes returns the maximum allowed write payload size in bytes.
func (c FilesystemConfig) MaxWriteBytes() int64 {
	return c.maxWriteBytes
}

// MaxListEntries returns the maximum number of list_directory entries.
func (c FilesystemConfig) MaxListEntries() int {
	return c.maxListEntries
}

// MaxCopyBytes returns the maximum allowed copy source size in bytes.
func (c FilesystemConfig) MaxCopyBytes() int64 {
	return c.maxCopyBytes
}

// MaxCopyEntries returns the maximum number of entries traversed by copy_path.
func (c FilesystemConfig) MaxCopyEntries() int {
	return c.maxCopyEntries
}

// MaxDeleteEntries returns the maximum entries allowed for recursive delete.
func (c FilesystemConfig) MaxDeleteEntries() int {
	return c.maxDeleteEntries
}

// AllowHiddenFiles returns whether hidden files may be accessed.
func (c SecurityConfig) AllowHiddenFiles() bool {
	return c.allowHiddenFiles
}

// AllowUNCPaths returns whether UNC paths may be used on Windows.
func (c SecurityConfig) AllowUNCPaths() bool {
	return c.allowUNCPaths
}

// FollowSymlinks returns whether symbolic links may be followed.
func (c SecurityConfig) FollowSymlinks() bool {
	return c.followSymlinks
}

// Name returns the server name.
func (c ServerConfig) Name() string {
	return c.name
}

// Version returns the server version.
func (c ServerConfig) Version() string {
	return c.version
}

// Debug returns whether debug mode is enabled.
func (c ServerConfig) Debug() bool {
	return c.debug
}

// MaxMessageBytes returns the maximum allowed JSON-RPC message size.
func (c ServerConfig) MaxMessageBytes() int64 {
	return c.maxMessageBytes
}

// MaxArgumentBytes returns the maximum allowed JSON-RPC tool argument size.
func (c ServerConfig) MaxArgumentBytes() int64 {
	return c.maxArgumentBytes
}

// MaxResponseBytes returns the maximum allowed JSON-RPC response size.
func (c ServerConfig) MaxResponseBytes() int64 {
	return c.maxResponseBytes
}
