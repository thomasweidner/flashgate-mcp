package operation

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

var (
	// ErrResultUnavailable deliberately combines missing, expired, and
	// ownership-mismatch outcomes so callers cannot probe stored results.
	ErrResultUnavailable = errors.New("operation result unavailable")
	ErrResultLimit       = errors.New("operation result storage limit exceeded")
)

// ResultStoreConfig defines hard storage and lifetime limits. All limits must
// be positive; callers must choose policy values explicitly.
type ResultStoreConfig struct {
	MaxEntries     int
	MaxTotalBytes  uint64
	MaxResultBytes uint64
	MaxTTL         time.Duration
}

// ResultBinding is the complete authorization context retained with a result.
// Possession of an operation handle is never sufficient for retrieval.
type ResultBinding struct {
	Principal         string
	Profile           string
	Root              string
	ExecutionBackend  string
	ServiceGeneration string
	Domain            string
}

// StoredResult is an immutable snapshot returned after authorization.
type StoredResult struct {
	Data      []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

type resultEntry struct {
	binding ResultBinding
	result  StoredResult
}

// ResultStore retains bounded operation results in memory. It is safe for
// concurrent use.
type ResultStore struct {
	mu         sync.Mutex
	config     ResultStoreConfig
	now        func() time.Time
	entries    map[string]resultEntry
	totalBytes uint64
}

// NewResultStore constructs an empty store. It does not select policy defaults.
func NewResultStore(config ResultStoreConfig) (*ResultStore, error) {
	return newResultStore(config, time.Now)
}

func newResultStore(config ResultStoreConfig, now func() time.Time) (*ResultStore, error) {
	if config.MaxEntries <= 0 || config.MaxTotalBytes == 0 || config.MaxResultBytes == 0 ||
		config.MaxResultBytes > config.MaxTotalBytes || config.MaxTTL <= 0 || now == nil {
		return nil, errors.New("invalid operation result store configuration")
	}
	return &ResultStore{config: config, now: now, entries: make(map[string]resultEntry)}, nil
}

// Put stores one result for an opaque operation handle until its fixed expiry.
// Existing handles are rejected rather than overwritten.
func (s *ResultStore) Put(handle string, binding ResultBinding, data []byte, ttl time.Duration) error {
	if err := validateHandleAndBinding(handle, binding); err != nil {
		return err
	}
	if ttl <= 0 || ttl > s.config.MaxTTL {
		return fmt.Errorf("%w: invalid TTL", ErrResultLimit)
	}
	size := uint64(len(data))
	if size > s.config.MaxResultBytes {
		return fmt.Errorf("%w: result bytes", ErrResultLimit)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.removeExpiredLocked(now)
	if _, exists := s.entries[handle]; exists {
		return errors.New("operation result already stored")
	}
	if len(s.entries) >= s.config.MaxEntries || size > math.MaxUint64-s.totalBytes || s.totalBytes+size > s.config.MaxTotalBytes {
		return fmt.Errorf("%w: store capacity", ErrResultLimit)
	}
	copyData := append([]byte(nil), data...)
	s.entries[handle] = resultEntry{
		binding: binding,
		result:  StoredResult{Data: copyData, CreatedAt: now, ExpiresAt: now.Add(ttl)},
	}
	s.totalBytes += size
	return nil
}

// Get returns a copy only when the complete binding matches and the result has
// not expired. Retrieval never extends the TTL.
func (s *ResultStore) Get(handle string, binding ResultBinding) (StoredResult, error) {
	if err := validateHandleAndBinding(handle, binding); err != nil {
		return StoredResult{}, ErrResultUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	entry, ok := s.entries[handle]
	if !ok || !entry.result.ExpiresAt.After(now) || entry.binding != binding {
		if ok && !entry.result.ExpiresAt.After(now) {
			s.removeLocked(handle, entry)
		}
		return StoredResult{}, ErrResultUnavailable
	}
	result := entry.result
	result.Data = append([]byte(nil), entry.result.Data...)
	return result, nil
}

// Delete removes a result only for its owner. Missing and mismatched results
// have the same response.
func (s *ResultStore) Delete(handle string, binding ResultBinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[handle]
	now := s.now()
	if !ok || !entry.result.ExpiresAt.After(now) || entry.binding != binding {
		if ok && !entry.result.ExpiresAt.After(now) {
			s.removeLocked(handle, entry)
		}
		return ErrResultUnavailable
	}
	s.removeLocked(handle, entry)
	return nil
}

// SweepExpired removes expired results and returns the number and bytes freed.
func (s *ResultStore) SweepExpired() (int, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeExpiredLocked(s.now())
}

func (s *ResultStore) removeExpiredLocked(now time.Time) (int, uint64) {
	var count int
	var bytes uint64
	for handle, entry := range s.entries {
		if !entry.result.ExpiresAt.After(now) {
			count++
			bytes += uint64(len(entry.result.Data))
			s.removeLocked(handle, entry)
		}
	}
	return count, bytes
}

func (s *ResultStore) removeLocked(handle string, entry resultEntry) {
	delete(s.entries, handle)
	s.totalBytes -= uint64(len(entry.result.Data))
}

func validateHandleAndBinding(handle string, binding ResultBinding) error {
	if !strings.HasPrefix(handle, "op_") || len(handle) <= len("op_") {
		return errors.New("invalid operation handle")
	}
	values := [...]string{binding.Principal, binding.Profile, binding.Root, binding.ExecutionBackend, binding.ServiceGeneration, binding.Domain}
	for _, value := range values {
		if value == "" || strings.TrimSpace(value) != value {
			return errors.New("invalid operation result binding")
		}
	}
	return nil
}
