package tools

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/search"
)

const (
	maxSearchPageSize       = 100
	maxSearchCursorSessions = 128
	searchCursorTTL         = 5 * time.Minute
)

var (
	errSearchCursorInvalid = errors.New("invalid or stale search cursor")
	errSearchCursorLimit   = errors.New("search cursor limit exceeded")
)

type searchCursorPage struct {
	paths     []search.Path
	matches   []search.LiteralMatch
	truncated bool
	limit     *search.MatchLimitDiagnostic
	skipped   []search.ContentSkip
	pageSize  int
	expiresAt time.Time
}

type searchCursorStore struct {
	mu      sync.Mutex
	pages   map[string]searchCursorPage
	now     func() time.Time
	ttl     time.Duration
	maximum int
}

func newSearchCursorStore() *searchCursorStore {
	return &searchCursorStore{pages: make(map[string]searchCursorPage), now: time.Now, ttl: searchCursorTTL, maximum: maxSearchCursorSessions}
}

func (s *searchCursorStore) put(page searchCursorPage) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for token, candidate := range s.pages {
		if !candidate.expiresAt.After(now) {
			delete(s.pages, token)
		}
	}
	if len(s.pages) >= s.maximum {
		return "", errSearchCursorLimit
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	page.expiresAt = now.Add(s.ttl)
	s.pages[token] = page
	return token, nil
}

// take makes cursors single-use. A successful continuation receives a fresh
// cursor, preventing replay and concurrent consumers from forking one page.
func (s *searchCursorStore) take(token string) (searchCursorPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	page, ok := s.pages[token]
	if !ok {
		return searchCursorPage{}, errSearchCursorInvalid
	}
	delete(s.pages, token)
	if !page.expiresAt.After(s.now()) {
		return searchCursorPage{}, errSearchCursorInvalid
	}
	return page, nil
}
