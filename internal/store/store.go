package store

import (
	"regexp"
	"sync"

	"github.com/tcontardo/prettylogs/internal/record"
)

const DefaultMaxSize = 50_000

type Store struct {
	mu       sync.RWMutex
	records  []*record.Record
	filtered []*record.Record
	maxSize  int
	nextID   uint64
	level    string
	pattern  *regexp.Regexp
	query    string
	queryErr error
}

func New(maxSize int) *Store {
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	return &Store{maxSize: maxSize}
}

func (s *Store) Add(rec record.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	rec.ID = s.nextID
	ptr := &rec
	if len(s.records) >= s.maxSize {
		oldest := s.records[0]
		s.records = s.records[1:]
		s.filtered = removeID(s.filtered, oldest.ID)
	}
	s.records = append(s.records, ptr)
	if s.matches(ptr) {
		s.filtered = append(s.filtered, ptr)
	}
}

func (s *Store) Filtered() []*record.Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*record.Record, len(s.filtered))
	copy(out, s.filtered)
	return out
}

func (s *Store) MaxID() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextID
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

func (s *Store) Counts() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := map[string]int{
		record.LevelError: 0,
		record.LevelWarn:  0,
		record.LevelInfo:  0,
		record.LevelDebug: 0,
	}
	for _, r := range s.records {
		counts[r.Level]++
	}
	return counts
}

func (s *Store) Level() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.level == "" {
		return "All"
	}
	return s.level
}

func (s *Store) Query() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.query
}

func (s *Store) QueryError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queryErr
}

// Pattern returns the currently active search regexp, or nil if there is
// no search query or the query failed to compile.
func (s *Store) Pattern() *regexp.Regexp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pattern
}

func (s *Store) SetLevel(level string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if level == "All" {
		level = ""
	}
	s.level = level
	s.rebuild()
}

func (s *Store) SetSearch(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.query = query
	s.pattern = nil
	s.queryErr = nil
	if query != "" {
		re, err := regexp.Compile(query)
		if err != nil {
			s.queryErr = err
		} else {
			s.pattern = re
		}
	}
	s.rebuild()
}

func (s *Store) rebuild() {
	s.filtered = s.filtered[:0]
	for _, r := range s.records {
		if s.matches(r) {
			s.filtered = append(s.filtered, r)
		}
	}
}

func (s *Store) matches(r *record.Record) bool {
	if s.level != "" && r.Level != s.level {
		return false
	}
	if s.query == "" {
		return true
	}
	if s.queryErr != nil || s.pattern == nil {
		return false
	}
	return s.pattern.MatchString(r.Raw) || s.pattern.MatchString(r.Message)
}

func removeID(in []*record.Record, id uint64) []*record.Record {
	if len(in) > 0 && in[0].ID == id {
		return in[1:]
	}
	for i, r := range in {
		if r.ID == id {
			return append(in[:i], in[i+1:]...)
		}
	}
	return in
}
