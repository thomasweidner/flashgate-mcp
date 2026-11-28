package audit

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

const (
	lockName        = "store.lock"
	leaseName       = "store.lease"
	maxStoreEntries = security.AuditTreeMaxEntries
	epochSize       = 27
	clockFormat     = "200601021504"
)

type StoreLimits struct {
	Bytes        int64
	SegmentBytes int64
	Retention    time.Duration
}

func (l StoreLimits) Valid() bool {
	return l.SegmentBytes >= 4<<10 && l.SegmentBytes <= 256<<20 &&
		l.Bytes >= l.SegmentBytes+4096 && l.Bytes <= 1<<40 &&
		l.Retention >= 24*time.Hour && l.Retention <= 365*24*time.Hour
}

type segment struct {
	name string
	day  time.Time
	info os.FileInfo
}

// FileStore is called by one recorder worker. The native lock serializes
// structural changes, budget admission and writes across workers.
type FileStore struct {
	path    string
	limits  StoreLimits
	binding []byte
	owner   []byte
	header  []byte
	root    *os.Root
	anchor  os.FileInfo
	lock    *os.File
	lease   *os.File
	leased  bool
	active  *os.File
	name    string
	epoch   string
	clock   time.Time
	seen    time.Time
	entries []segment
	now     func() time.Time
}

func NewFileStore(path, serverBinding string, limits StoreLimits) (*FileStore, error) {
	if !limits.Valid() || !label(serverBinding) || !filepath.IsAbs(path) ||
		filepath.Clean(path) != path || filepath.Dir(path) == path {
		return nil, ErrLimits
	}
	return &FileStore{
		path: path, limits: limits, now: time.Now,
		owner: fmt.Appendf(nil, "flashgate-audit-1:%s:", serverBinding),
		binding: fmt.Appendf(nil, "flashgate-audit-1:%s:%013d:%09d:%020d\n", serverBinding, limits.Bytes,
			limits.SegmentBytes, limits.Retention),
		header: fmt.Appendf(nil, "{\"v\":1,\"phase\":\"segment\",\"store\":\"%s\"}\n", serverBinding),
	}, nil
}

func (s *FileStore) open() error {
	if s.root != nil {
		return nil
	}
	parentName := filepath.Dir(s.path)
	resolved, err := filepath.EvalSymlinks(parentName)
	if err != nil || !nativeSamePath(resolved, parentName) {
		return ErrUnavailable
	}
	parent, err := os.OpenRoot(parentName)
	if err != nil {
		return err
	}
	defer parent.Close()
	leaf := filepath.Base(s.path)
	if err := makeStoreDirectory(parent, leaf); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	dir, err := secureOpen(parent, leaf, os.O_RDONLY, 0, true)
	if err != nil {
		return err
	}
	defer dir.Close()
	anchor, err := dir.Stat()
	if err != nil {
		return err
	}
	root, err := parent.OpenRoot(leaf)
	if err != nil {
		return err
	}
	if err := matchNode(root, ".", anchor, true); err != nil {
		root.Close()
		return err
	}
	lock, err := openOrCreate(root, lockName)
	if err != nil {
		root.Close()
		return err
	}
	lease, err := openOrCreate(root, leaseName)
	if err != nil {
		lock.Close()
		root.Close()
		return err
	}
	s.root, s.anchor, s.lock, s.lease = root, anchor, lock, lease
	return nil
}

func openOrCreate(root *os.Root, name string) (*os.File, error) {
	file, err := createStoreFile(root, name)
	if errors.Is(err, os.ErrExist) {
		return secureOpen(root, name, os.O_RDWR, 0, false)
	}
	return file, err
}

func matchNode(root *os.Root, name string, expected os.FileInfo, directory bool) error {
	file, err := secureOpen(root, name, os.O_RDONLY, 0, directory)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !os.SameFile(expected, info) {
		return ErrUnavailable
	}
	return nil
}

func awaitLock(ctx context.Context, file *os.File) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if acquired, err := lockFile(file, true); err != nil || acquired {
		return err
	}
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		acquired, err := lockFile(file, true)
		if err != nil || acquired {
			return err
		}
	}
}

func (s *FileStore) withLock(ctx context.Context, use func() error) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := awaitLock(ctx, s.lock); err != nil {
		return err
	}
	defer unlockFile(s.lock)
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(s.path)
	if err != nil || !os.SameFile(s.anchor, info) {
		return ErrUnavailable
	}
	current, err := s.lock.Stat()
	// matchNode verifies native ownership, link count and permissions on the
	// current path and checks that it identifies the handle holding the lock.
	if err != nil || matchNode(s.root, lockName, current, false) != nil {
		return ErrUnavailable
	}
	lease, err := s.lease.Stat()
	if err != nil || lease.Size() != 0 || matchNode(s.root, leaseName, lease, false) != nil {
		return ErrUnavailable
	}
	if err := s.refresh(); err != nil {
		return err
	}
	return use()
}

func (s *FileStore) refresh() error {
	var data [256]byte
	n, err := s.lock.ReadAt(data[:], 0)
	if err != nil && !errors.Is(err, io.EOF) || n == len(data) {
		return ErrUnavailable
	}
	if n == 0 {
		entries, scanErr := s.scan()
		if scanErr != nil || len(entries) != 0 {
			return ErrUnavailable
		}
		if _, err := s.lock.WriteAt(s.binding, 0); err != nil {
			return err
		}
		if err := s.setEpoch(); err != nil {
			return err
		}
		return s.claimLease()
	}
	if n < len(s.binding) {
		return ErrUnavailable
	}
	marker := data[len(s.binding):n]
	switch len(marker) {
	case epochSize:
		if !s.clock.IsZero() {
			return ErrUnavailable
		}
	case epochSize + len(clockFormat):
		stamp, err := time.Parse(clockFormat, string(marker[epochSize:]))
		if err != nil || stamp.Before(s.clock) {
			return ErrUnavailable
		}
		s.clock = stamp
		marker = marker[:epochSize]
	default:
		return ErrUnavailable
	}
	matches := bytes.HasPrefix(data[:n], s.binding)
	if !matches {
		if !bytes.HasPrefix(data[:n], s.owner) {
			return ErrUnavailable
		}
		if err := s.changeLimits(); err != nil {
			return err
		}
		return s.claimLease()
	}
	if err := s.claimLease(); err != nil {
		return err
	}
	next := string(marker)
	if next == s.epoch && next != "" {
		return nil
	}
	s.closeActive()
	s.entries, err = s.scan()
	if err != nil {
		return err
	}
	if len(next) != epochSize || next[0] != 'C' || !label(next[1:]) {
		return s.setEpoch()
	}
	s.epoch = next
	return nil
}

func (s *FileStore) claimLease() error {
	if s.leased {
		return nil
	}
	acquired, err := lockFile(s.lease, false)
	if err != nil || !acquired {
		return ErrUnavailable
	}
	s.leased = true
	return nil
}

func (s *FileStore) changeLimits() error {
	// Hold the writer lock while testing whether any older runtime still
	// uses the bound policy. This also serializes the exclusive-to-shared
	// transition with every cooperating instance.
	if s.leased {
		return ErrUnavailable
	}
	acquired, err := lockFile(s.lease, true)
	if err != nil || !acquired {
		return ErrUnavailable
	}
	defer unlockFile(s.lease)
	s.closeActive()
	s.entries, err = s.scan()
	if err != nil {
		return err
	}
	now, err := s.retentionTime()
	if err != nil {
		return err
	}
	if _, err := s.prune(now, 0, false, nil); err != nil {
		return err
	}
	if err := s.invalidate(); err != nil {
		return err
	}
	if _, err := s.lock.WriteAt(s.binding, 0); err != nil {
		return err
	}
	return s.setEpoch()
}

// retentionTime persists an observed UTC minute across writers and restarts.
// The in-memory high-water mark also preserves finer progress within a minute.
// Call it with the writer lock held and the coordination file refreshed.
func (s *FileStore) retentionTime() (time.Time, error) {
	now := s.now().UTC()
	if n := len(s.entries); n > 0 && s.entries[n-1].day.After(now) {
		now = s.entries[n-1].day
	}
	if s.clock.After(now) {
		now = s.clock
	}
	if s.seen.After(now) {
		now = s.seen
	}
	stamp := now.Truncate(time.Minute)
	if stamp.After(s.clock) {
		raw := stamp.Format(clockFormat)
		if len(raw) != len(clockFormat) {
			return time.Time{}, ErrUnavailable
		}
		if _, err := s.lock.WriteAt([]byte(raw), int64(len(s.binding)+epochSize)); err != nil {
			return time.Time{}, err
		}
		if err := s.lock.Sync(); err != nil {
			return time.Time{}, err
		}
		s.clock = stamp
	}
	s.seen = now
	return now, nil
}

func (s *FileStore) invalidate() error {
	s.epoch = ""
	_, err := s.lock.WriteAt([]byte{'D'}, int64(len(s.binding)))
	if err != nil {
		return err
	}
	return s.lock.Sync()
}

func (s *FileStore) setEpoch() error {
	next := "C" + rand.Text()
	if len(next) != epochSize {
		return ErrUnavailable
	}
	if _, err := s.lock.WriteAt([]byte(next), int64(len(s.binding))); err != nil {
		return err
	}
	if err := s.lock.Sync(); err != nil {
		return err
	}
	s.epoch = next
	return nil
}

func (s *FileStore) scan() ([]segment, error) {
	var entries []segment
	remaining := maxStoreEntries
	var walk func(string, int) error
	walk = func(name string, depth int) error {
		dir, err := secureOpen(s.root, name, os.O_RDONLY, 0, true)
		if err != nil {
			return err
		}
		defer dir.Close()
		items, err := dir.ReadDir(remaining + 1)
		if err != nil && !errors.Is(err, io.EOF) || len(items) > remaining {
			return ErrUnavailable
		}
		remaining -= len(items)
		for _, item := range items {
			part := item.Name()
			if depth == 0 && (part == lockName || part == leaseName) {
				continue
			}
			child := filepath.Join(name, part)
			if depth < 3 && item.IsDir() && digits(part, []int{4, 2, 2}[depth]) {
				if err := walk(child, depth+1); err != nil {
					return err
				}
				continue
			}
			slot, ok := parseSegment(child)
			if depth != 3 || !ok {
				return ErrUnavailable
			}
			file, err := secureOpen(s.root, child, os.O_RDONLY, 0, false)
			if err != nil {
				return err
			}
			info, infoErr := file.Stat()
			valid := s.ownsSegment(file)
			file.Close()
			if !valid || infoErr != nil {
				return ErrUnavailable
			}
			slot.info = info
			entries = append(entries, slot)
		}
		return nil
	}
	if err := walk(".", 0); err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b segment) int { return strings.Compare(a.name, b.name) })
	return entries, nil
}

func digits(s string, length int) bool {
	return len(s) == length && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

func parseSegment(name string) (segment, bool) {
	normalized := filepath.ToSlash(name)
	parts := strings.Split(normalized, "/")
	if len(parts) != 4 {
		return segment{}, false
	}
	day, err := time.Parse("2006/01/02", strings.Join(parts[:3], "/"))
	base, prefix := strings.CutPrefix(parts[3], "audit-")
	number, suffix := strings.CutSuffix(base, ".jsonl")
	return segment{name: name, day: day}, err == nil && prefix && suffix && digits(number, 6) && number != "000000"
}

func (s *FileStore) ownsSegment(file *os.File) bool {
	var prefix [256]byte
	n, err := file.ReadAt(prefix[:len(s.header)], 0)
	return err == nil && bytes.Equal(prefix[:n], s.header)
}

func (s *FileStore) putDay(day time.Time) (string, error) {
	dir := "."
	for _, part := range strings.Split(day.Format("2006/01/02"), "/") {
		child := filepath.Join(dir, part)
		if err := makeStorePartition(s.root, child); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		handle, err := secureOpen(s.root, child, os.O_RDONLY, 0, true)
		if err != nil {
			return "", err
		}
		handle.Close()
		if err := syncDirectory(s.root, dir); err != nil {
			return "", err
		}
		dir = child
	}
	return dir, nil
}

func (s *FileStore) closeActive() {
	if s.active != nil {
		s.active.Close()
		s.active, s.name = nil, ""
	}
}

func (s *FileStore) Close() error {
	s.closeActive()
	if s.root == nil {
		return nil
	}
	err := errors.Join(s.lock.Close(), s.lease.Close(), s.root.Close())
	s.root, s.lock, s.lease, s.leased = nil, nil, nil, false
	return err
}
