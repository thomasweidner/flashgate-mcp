package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

var errUnrepairableTail = errors.New("unrepairable audit segment tail")

func (s *FileStore) Maintain(ctx context.Context) error {
	return s.withLock(ctx, func() error {
		now, err := s.retentionTime()
		if err != nil {
			return err
		}
		_, err = s.prune(now, 0, false, nil)
		return err
	})
}

func (s *FileStore) Append(ctx context.Context, lines [][]byte, durable bool) error {
	records := make([]record, len(lines))
	for i, line := range lines {
		records[i].data = line
	}
	return s.appendRecords(ctx, records, durable, nil)
}

func recordDay(entry record, now time.Time) time.Time {
	if entry.at.IsZero() || entry.at.After(now) {
		return now.Truncate(24 * time.Hour)
	}
	return entry.at.UTC().Truncate(24 * time.Hour)
}

func (s *FileStore) appendRecords(ctx context.Context, records []record, durable bool, confirm func()) error {
	return s.withLock(ctx, func() error {
		now, err := s.retentionTime()
		if err != nil {
			return err
		}
		var protected map[string]struct{}
		if durable {
			protected = make(map[string]struct{})
		}
		for len(records) != 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			first := records[0]
			if len(first.data) == 0 || int64(len(first.data)+len(s.header)) > s.limits.SegmentBytes {
				return ErrLimits
			}
			day := recordDay(first, now)
			if !now.Before(day.Add(s.limits.Retention)) {
				return ErrUnavailable
			}
			at, err := s.selectSegment(day)
			if err != nil {
				return err
			}
			if s.active == nil || at+int64(len(first.data)) > s.limits.SegmentBytes {
				if err := s.rotate(now, day, int64(len(first.data)), protected); err != nil {
					return err
				}
				at = int64(len(s.header))
			}
			capacity := s.limits.SegmentBytes - at
			var size, count int
			for _, entry := range records {
				if len(entry.data) == 0 || int64(size)+int64(len(entry.data)) > capacity ||
					recordDay(entry, now) != day {
					break
				}
				size += len(entry.data)
				count++
			}
			if count == 0 {
				return ErrLimits
			}
			if _, err := s.prune(now, int64(size), true, protected); err != nil {
				return err
			}
			if err := s.appendLines(ctx, records[:count], at, size, durable); err != nil {
				return err
			}
			if durable {
				protected[s.name] = struct{}{}
			}
			records = records[count:]
		}
		if confirm != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Publish acknowledgements before a competing writer can prune.
			confirm()
		}
		return nil
	})
}

func (s *FileStore) selectSegment(day time.Time) (int64, error) {
	if len(s.entries) == 0 {
		return 0, nil
	}
	last := s.entries[len(s.entries)-1]
	if last.day != day {
		s.closeActive()
		return 0, nil
	}
	if s.active == nil || s.name != last.name {
		s.closeActive()
		file, err := secureOpen(s.root, last.name, os.O_RDWR, 0, false)
		if err != nil {
			return 0, err
		}
		info, err := file.Stat()
		if err != nil || !os.SameFile(info, last.info) || !s.ownsSegment(file) {
			file.Close()
			return 0, ErrUnavailable
		}
		s.active, s.name = file, last.name
	}
	info, err := s.active.Stat()
	if err != nil || matchNode(s.root, s.name, info, false) != nil {
		return 0, ErrUnavailable
	}
	if info.Size() > s.limits.SegmentBytes {
		s.closeActive()
		return 0, nil // A prior, larger policy's segment is now closed.
	}
	size, err := s.recoverTail(info.Size())
	if err != nil {
		s.closeActive()
		if errors.Is(err, errUnrepairableTail) {
			return 0, nil
		}
		return 0, err
	}
	s.entries[len(s.entries)-1].info, err = s.active.Stat()
	return size, err
}

func (s *FileStore) recoverTail(size int64) (int64, error) {
	if size < int64(len(s.header)) || size > s.limits.SegmentBytes {
		return 0, ErrUnavailable
	}
	// A typical batch needs only one page of tail I/O under the writer lock.
	// Expand to the original repair bound when a complete record is unproved.
	limit := min(size, 64<<10+1)
	var tail []byte
	var last, start int
	for length := min(limit, 4<<10); ; length = limit {
		tail = make([]byte, length)
		if _, err := s.active.ReadAt(tail, size-length); err != nil {
			return 0, err
		}
		last = bytes.LastIndexByte(tail, '\n')
		start = 0
		if last >= 0 {
			start = bytes.LastIndexByte(tail[:last], '\n') + 1
		}
		if length == limit || last >= 0 && (start != 0 || length == size) {
			break
		}
	}
	if last < 0 || start == 0 && size != int64(len(tail)) ||
		!json.Valid(tail[start:last]) || tail[start] != '{' {
		return 0, errUnrepairableTail
	}
	end := size - int64(len(tail)-last-1)
	if end != size {
		if err := s.active.Truncate(end); err != nil {
			return 0, err
		}
		if err := s.active.Sync(); err != nil {
			return 0, err
		}
	}
	return end, nil
}

func (s *FileStore) appendLines(ctx context.Context, records []record, at int64, size int, durable bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	batch := make([]byte, 0, size)
	for _, entry := range records {
		batch = append(batch, entry.data...)
	}
	_, err := s.active.WriteAt(batch, at)
	return s.finishAppend(err, durable)
}

func (s *FileStore) finishAppend(err error, durable bool) error {
	var info os.FileInfo
	if err == nil {
		info, err = s.active.Stat()
	}
	if err != nil {
		// A short write or failed size readback can leave bytes in an older
		// segment. Dirty the shared inventory before releasing the writer lock
		// so the next admission rescans the store, including that segment.
		s.closeActive()
		return errors.Join(err, s.invalidate())
	}
	for i := range s.entries {
		if s.entries[i].name == s.name {
			s.entries[i].info = info
			break
		}
	}
	if durable {
		return s.active.Sync()
	}
	return nil
}

func (s *FileStore) rotate(now, day time.Time, need int64, protected map[string]struct{}) error {
	s.closeActive()
	if _, err := s.prune(now, int64(len(s.header))+need, false, protected); err != nil {
		return err
	}
	if len(s.entries) >= maxStoreEntries/4 {
		return ErrUnavailable
	}
	var sequence int64
	for _, entry := range s.entries {
		if entry.day == day {
			number := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(entry.name), "audit-"), ".jsonl")
			n, err := strconv.ParseInt(number, 10, 32)
			if err != nil {
				return ErrUnavailable
			}
			sequence = max(sequence, n)
		}
	}
	if sequence >= 999999 {
		return ErrUnavailable
	}
	if err := s.invalidate(); err != nil {
		return err
	}
	dir, err := s.putDay(day)
	if err != nil {
		return err
	}
	name := filepath.Join(dir, fmt.Sprintf("audit-%06d.jsonl", sequence+1))
	file, err := createStoreFile(s.root, name)
	if err != nil {
		return err
	}
	defer func() {
		if s.active != file {
			file.Close()
		}
	}()
	if _, err := file.Write(s.header); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := syncDirectory(s.root, dir); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	s.entries = append(s.entries, segment{name, day, info})
	slices.SortFunc(s.entries, func(a, b segment) int { return strings.Compare(a.name, b.name) })
	if err := s.setEpoch(); err != nil {
		return err
	}
	s.active, s.name = file, name
	return nil
}

func (s *FileStore) prune(now time.Time, requested int64, keepActive bool, protected map[string]struct{}) (int64, error) {
	total := int64(len(s.binding) + epochSize + len(clockFormat))
	for _, entry := range s.entries {
		total += entry.info.Size()
	}
	if len(s.entries) > 0 {
		last := &s.entries[len(s.entries)-1]
		file, err := secureOpen(s.root, last.name, os.O_RDONLY, 0, false)
		if err != nil {
			return 0, err
		}
		current, err := file.Stat()
		file.Close()
		if err != nil || !os.SameFile(last.info, current) || current.Size() > 256<<20 {
			return 0, ErrUnavailable
		}
		total += current.Size() - last.info.Size()
		last.info = current
	}
	dirty := false
	for i := 0; i < len(s.entries); {
		entry := s.entries[i]
		expired := !now.Before(entry.day.Add(s.limits.Retention))
		_, inFlight := protected[entry.name]
		if inFlight || keepActive && entry.name == s.name || !expired && total+requested <= s.limits.Bytes {
			i++
			continue
		}
		if !dirty {
			if err := s.invalidate(); err != nil {
				return 0, err
			}
			dirty = true
		}
		if entry.name == s.name {
			s.closeActive()
		}
		if err := s.removeSegment(entry); err != nil {
			return 0, err
		}
		total -= entry.info.Size()
		s.entries = slices.Delete(s.entries, i, i+1)
	}
	if dirty {
		if err := s.setEpoch(); err != nil {
			return 0, err
		}
	}
	if total+requested > s.limits.Bytes {
		return 0, ErrUnavailable
	}
	return total, nil
}

func (s *FileStore) removeSegment(entry segment) error {
	if err := matchNode(s.root, entry.name, entry.info, false); err != nil {
		return err
	}
	if err := s.root.Remove(entry.name); err != nil {
		return err
	}
	dir := filepath.Dir(entry.name)
	for dir != "." {
		handle, err := secureOpen(s.root, dir, os.O_RDONLY, 0, true)
		if err != nil {
			return err
		}
		names, err := handle.Readdirnames(1)
		handle.Close()
		if len(names) != 0 || !errors.Is(err, io.EOF) {
			return errIfNotEOF(err)
		}
		if err := s.root.Remove(dir); err != nil {
			return err
		}
		dir = filepath.Dir(dir)
	}
	return syncDirectory(s.root, ".")
}

func errIfNotEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
